package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDisplaySongProposal(t *testing.T) {
	for _, tc := range []struct {
		name        string
		titles      []string
		wantChanged bool
		want        []string
	}{
		{"different title", []string{"song"}, true, []string{`"song"`, `"Song"`, "changed"}},
		{"spacing in a repeated tag", []string{"Song", " Song "}, true, []string{`"Song", " Song "`, "changed"}},
		{"same tags", []string{"Song"}, false, []string{"The title and artist already agree."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := [][]byte{frame(4, "TPE1", []byte("\x03Artist"))}
			for _, title := range tc.titles {
				frames = append(frames, frame(4, "TIT2", []byte("\x03"+title)))
			}
			path := save(t, fixture(4, frames...))
			tags, err := readLookupTags(path)
			if err != nil {
				t.Fatal(err)
			}
			proposal := proposeSong(path, tags, recording{id: "example-id", title: "Song", artist: "Artist"})
			if proposal.changed() != tc.wantChanged {
				t.Errorf("proposal.changed() = %t, want %t", proposal.changed(), tc.wantChanged)
			}
			var output bytes.Buffer
			if err := displayProposal(&output, proposal); err != nil {
				t.Fatal(err)
			}
			for _, want := range append(tc.want, path, "example-id", "Current", "Proposed") {
				if !strings.Contains(output.String(), want) {
					t.Errorf("preview = %q, missing %q", output.String(), want)
				}
			}
		})
	}
}

func TestPreviewTerminal(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys []string
		wait []string
		want string
		code int
	}{
		{
			name: "change recording and accept", code: 0, want: "Accepted for this preview:",
			wait: []string{"? Choose a recording", "❯ Skip song", "? Choose a recording", "❯ Skip song"},
			keys: []string{"\x1b[B\r", "\x1b[B\x1b[B\r", "\x1b[A\r", "\x1b[B\r"},
		},
		{
			name: "default skip", code: 0, want: "Skipped song.",
			wait: []string{"? Choose a recording", "❯ Skip song"}, keys: []string{"\r", "\r"},
		},
		{
			name: "quit", code: 0, want: "Preview ended.",
			wait: []string{"? Choose a recording"}, keys: []string{"\x1b[A\r"},
		},
		{
			name: "cancel", code: 130,
			wait: []string{"? Choose a recording"}, keys: []string{"\x03"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := save(t, fixture(4, frame(4, "TIT2", []byte("\x03song")), frame(4, "TPE1", []byte("\x03Artist"))))
			before := inspectionSnapshot(t, path)
			var requests atomic.Int32
			client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if query := r.URL.Query().Get("query"); query != `recording:"song" AND artist:"Artist"` {
					t.Errorf("query = %q", query)
				}
				io.WriteString(w, `{"recordings":[
					{"id":"studio-id","title":"Song","artist-credit":[{"name":"Artist"}],"disambiguation":"studio","length":139000},
					{"id":"live-id","title":"Song (Live)","artist-credit":[{"name":"Artist"}],"disambiguation":"live","length":147000}
				]}`)
			})
			master, terminal := previewTestTerminal(t)
			originalMode, err := unix.IoctlGetTermios(int(terminal.Fd()), unix.TCGETS)
			if err != nil {
				t.Fatal(err)
			}
			var errOut bytes.Buffer
			exit := make(chan int, 1)
			go func() {
				exit <- runCommand([]string{"preview", path}, terminal, terminal, &errOut, client.search)
			}()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var transcript strings.Builder
			for i, want := range tc.wait {
				transcript.WriteString(readPreviewOutput(t, ctx, master, want))
				if tc.name == "change recording and accept" && i == 0 {
					transcript.WriteString(readPreviewOutput(t, ctx, master, "  Quit"))
					initial := transcript.String()
					if strings.Count(initial, "2. Song (Live) — Artist") != 1 ||
						!strings.Contains(initial, "Recording ID: studio-id") || strings.Contains(initial, "live-id") {
						t.Errorf("initial picker repeated a candidate or showed details for the wrong option: %q", initial)
					}
				}
				if _, err := io.WriteString(master, tc.keys[i]); err != nil {
					t.Fatal(err)
				}
			}
			if tc.want != "" {
				transcript.WriteString(readPreviewOutput(t, ctx, master, tc.want))
			}
			if tc.name == "change recording and accept" {
				transcript.WriteString(readPreviewOutput(t, ctx, master, "(recording studio-id)."))
				for _, want := range []string{"Version: live", "2:27", "Recording ID: live-id", "\x1b[J✓ 2. Song (Live)", "MusicBrainz recording: live-id", "Current", "Proposed", "changed"} {
					if !strings.Contains(transcript.String(), want) {
						t.Errorf("missing %q in terminal output", want)
					}
				}
			}
			select {
			case code := <-exit:
				if code != tc.code {
					t.Errorf("exit = %d, want %d; stderr = %q", code, tc.code, &errOut)
				}
			case <-ctx.Done():
				t.Fatal("preview did not exit")
			}
			restoredMode, err := unix.IoctlGetTermios(int(terminal.Fd()), unix.TCGETS)
			if err != nil || *restoredMode != *originalMode {
				t.Errorf("terminal mode was not restored: %v", err)
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("requests = %d, want 1", got)
			}
			if after := inspectionSnapshot(t, path); after != before {
				t.Errorf("input changed: before %+v, after %+v", before, after)
			}
		})
	}
}

func TestPreviewOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		artist        string
		status        int
		response      string
		want          string
		wantError     bool
		wantRequests  int32
		wantSelection bool
	}{
		{"missing artist", "", 200, "", "missing artist tag", true, 0, false},
		{"conflicting artist", "Artist\x00Other", 200, "", "conflicting artist tags", true, 0, false},
		{"empty results", "Artist", 200, `{"recordings":[]}`, "No MusicBrainz recording candidates found.", false, 1, false},
		{"service failure", "Artist", 503, "", "MusicBrainz is busy", true, 1, false},
		{"no changes", "Artist", 200, `{"recordings":[{"id":"recording","title":"Song","artist-credit":[{"name":"Artist"}]}]}`, "The title and artist already agree.", false, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := save(t, fixture(4, frame(4, "TIT2", []byte("\x03Song")), frame(4, "TPE1", []byte("\x03"+tc.artist))))
			var requests atomic.Int32
			client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.response)
			})
			var output bytes.Buffer
			selected := false
			err := previewFile(path, &output, client.search, func(_ context.Context, prompt string, _ []menuOption, _ int) (int, error) {
				if !tc.wantSelection || selected || prompt != "Choose a recording" {
					t.Fatalf("unexpected prompt %q", prompt)
				}
				selected = true
				return 0, nil
			})
			message := output.String()
			if err != nil {
				message = err.Error()
			}
			if (err != nil) != tc.wantError || !strings.Contains(message, tc.want) {
				t.Errorf("previewFile = (%q, %v), want %q", &output, err, tc.want)
			}
			if got := requests.Load(); got != tc.wantRequests {
				t.Errorf("requests = %d, want %d", got, tc.wantRequests)
			}
			if selected != tc.wantSelection {
				t.Errorf("candidate selection = %t, want %t", selected, tc.wantSelection)
			}
		})
	}
}

func TestPreviewRequiresTerminal(t *testing.T) {
	path := save(t, fixture(4, frame(4, "TIT2", []byte("\x03Song")), frame(4, "TPE1", []byte("\x03Artist"))))
	var out, errOut bytes.Buffer
	code := runCommand([]string{"preview", path}, strings.NewReader("\n"), &out, &errOut, func(_ context.Context, _, _ string) ([]recording, error) {
		t.Fatal("preview attempted a lookup without a terminal")
		return nil, nil
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "preview requires a terminal") {
		t.Errorf("exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
}

func previewTestTerminal(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("open pseudo-terminal: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	unit, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", unit), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { terminal.Close() })
	if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80}); err != nil {
		t.Fatal(err)
	}
	return master, terminal
}

func readPreviewOutput(t *testing.T, ctx context.Context, terminal *os.File, want string) string {
	t.Helper()
	var output strings.Builder
	fd := int(terminal.Fd())
	for !strings.HasSuffix(output.String(), want) {
		b, err := readMenuByte(ctx, fd)
		if err != nil {
			t.Fatalf("waiting for %q: %v; got %q", want, err, output.String())
		}
		output.WriteByte(b)
	}
	return output.String()
}
