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

func selectTerminalMenu(in, out *os.File, question string, options []string, selected int) (choice int, err error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, unix.SIGTERM)
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

	width, height, _ := term.GetSize(int(out.Fd()))
	if width == 0 || height == 0 {
		width, height = 80, 24
	}
	rows := min(len(options), max(1, height-2))
	if _, err := fmt.Fprintf(out, "\r\n%s\r\n", fitMenuLine("? "+question+"  (↑/↓, Enter)", width)); err != nil {
		return 0, err
	}
	for redraw := false; ; redraw = true {
		if err := drawMenu(out, options, selected, width, rows, redraw); err != nil {
			return 0, err
		}
		key, err := readMenuKey(ctx, fd)
		if err != nil {
			return 0, err
		}
		switch key {
		case '\r':
			return selected, nil
		case 'A':
			selected = (selected + len(options) - 1) % len(options)
		case 'B':
			selected = (selected + 1) % len(options)
		}
	}
}

func drawMenu(out io.Writer, options []string, selected, width, rows int, redraw bool) error {
	var text strings.Builder
	if redraw {
		fmt.Fprintf(&text, "\x1b[%dA", rows)
	}
	first := max(0, selected-rows+1)
	for i := first; i < first+rows; i++ {
		prefix := "  "
		if i == selected {
			prefix = "❯ "
		}
		fmt.Fprintf(&text, "\r\x1b[2K%s\r\n", fitMenuLine(prefix+options[i], width))
	}
	_, err := io.WriteString(out, text.String())
	return err
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
