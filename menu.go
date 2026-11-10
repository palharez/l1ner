package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

var errPreviewCanceled = errors.New("preview canceled")

type menuOption struct {
	label   string
	details []string
}

func selectTerminalMenu(ctx context.Context, in, out *os.File, question string, options []menuOption, selected int) (choice int, err error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	width, height, _ := term.GetSize(int(out.Fd()))
	if width == 0 || height == 0 {
		width, height = 80, 24
	}
	if width < 20 || height < 4 {
		return 0, errors.New("terminal is too small to display the menu")
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, unix.SIGTERM)
	defer stop()
	fd := int(in.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return 0, fmt.Errorf("set terminal input mode: %w", err)
	}
	defer func() {
		if restoreErr := term.Restore(fd, state); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore terminal input mode: %w", restoreErr))
		}
	}()

	color := os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	if _, err := io.WriteString(out, "\r\n"); err != nil {
		return 0, err
	}
	rows := 0
	for {
		lines := menuLines(question, options, selected, width, height, color)
		if err := replaceMenu(out, rows, lines); err != nil {
			return 0, err
		}
		rows = len(lines)
		key, err := readMenuKey(ctx, fd)
		if err != nil {
			if clearErr := replaceMenu(out, rows, nil); clearErr != nil {
				return 0, errors.Join(err, clearErr)
			}
			return 0, err
		}
		switch key {
		case '\r':
			answer := clipMenuLine("✓ "+options[selected].label, width)
			return selected, replaceMenu(out, rows, []string{answer})
		case 'A':
			selected = (selected + len(options) - 1) % len(options)
		case 'B':
			selected = (selected + 1) % len(options)
		}
	}
}

func menuLines(question string, options []menuOption, selected, width, height int, color bool) []string {
	lines := []string{clipMenuLine("? "+question+"  (↑/↓, Enter)", width)}
	details := wrapMenuDetails(options[selected].details, width, min(4, max(0, height-3)))
	rows := min(len(options), max(1, height-2-len(details)))
	first := min(max(0, selected-rows/2), len(options)-rows)
	for i := first; i < first+rows; i++ {
		if i != selected {
			lines = append(lines, clipMenuLine("  "+options[i].label, width))
			continue
		}
		line := clipMenuLine("❯ "+options[i].label, width)
		if color {
			line = "\x1b[36m" + line + "\x1b[0m"
		}
		lines = append(lines, line)
		lines = append(lines, details...)
	}
	return lines
}

func wrapMenuDetails(details []string, width, maxRows int) []string {
	const indent = "    "
	var lines []string
	for _, detail := range details {
		for detail != "" && len(lines) < maxRows {
			part := fitMenuLine(detail, width-len(indent))
			if part == "" {
				return lines
			}
			if len(part) < len(detail) {
				if len(lines)+1 == maxRows {
					return append(lines, clipMenuLine(indent+detail, width))
				}
				if space := strings.LastIndexByte(part, ' '); space > 0 {
					part = part[:space]
				}
			}
			lines = append(lines, indent+part)
			detail = strings.TrimLeft(detail[len(part):], " ")
		}
	}
	return lines
}

func replaceMenu(out io.Writer, rows int, lines []string) error {
	var text strings.Builder
	if rows > 0 {
		// The cursor sits one row below the prompt, so erase exactly from its start.
		fmt.Fprintf(&text, "\x1b[%dA\r\x1b[J", rows)
	}
	for _, line := range lines {
		text.WriteString(line)
		text.WriteString("\r\n")
	}
	_, err := io.WriteString(out, text.String())
	return err
}

func clipMenuLine(text string, width int) string {
	if line := fitMenuLine(text, width); len(line) == len(text) || width < 5 {
		return line
	}
	return fitMenuLine(text, width-3) + "..."
}

func fitMenuLine(text string, width int) string {
	// Leave the last column free. Counting non-ASCII runes as two cells avoids
	// wraps even in terminals that render ambiguous-width characters as wide.
	remaining := width - 1
	for index, r := range text {
		cells := 1
		if r >= 128 {
			cells = 2
		}
		if cells > remaining {
			return text[:index]
		}
		remaining -= cells
	}
	return text
}

func readMenuKey(ctx context.Context, fd int) (byte, error) {
	const escape = 27
	var prefix byte
	for {
		key, err := readMenuByte(ctx, fd)
		if err != nil {
			return 0, err
		}
		switch {
		case key == '\r' || key == '\n':
			return '\r', nil
		case key == 3 || key == 4: // Ctrl+C or Ctrl+D in raw mode.
			return 0, errPreviewCanceled
		case key == escape:
			prefix = escape
		case prefix == escape && (key == '[' || key == 'O'):
			prefix = '['
		case prefix == '[' && key >= '@' && key <= '~':
			if key == 'A' || key == 'B' {
				return key, nil
			}
			prefix = 0
		case prefix == escape:
			prefix = 0
		}
	}
}

func readMenuByte(ctx context.Context, fd int) (byte, error) {
	var data [1]byte
	descriptors := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for ctx.Err() == nil {
		// Poll so an external interrupt can restore the terminal without waiting
		// for another key. A typed Ctrl+C is handled as an input byte in raw mode.
		ready, err := unix.Poll(descriptors, 100)
		if errors.Is(err, unix.EINTR) || (err == nil && ready == 0) {
			continue
		}
		if err != nil {
			return 0, err
		}
		n, err := unix.Read(fd, data[:])
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		return data[0], nil
	}
	return 0, errPreviewCanceled
}
