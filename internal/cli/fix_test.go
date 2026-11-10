package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFixTerminalApproval(t *testing.T) {
	for _, tc := range []struct {
		name, keys, want string
		chooseRecording  bool
		code             int
	}{
		{"apply", "\x1b[B\r", "Backup:", true, 0},
		{"skip by default", "\r", "Skipped song.", true, 0},
		{"quit", "\x1b[A\r", "Review ended.", false, 0},
		{"cancel", "\x03", "", true, 130},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, original := saveFixFixture(t)
			client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"recordings":[{"id":"studio","title":"Title","artist-credit":[{"name":"Artist"}]}]}`)
			})
			master, terminal := previewTestTerminal(t)
			var errOut bytes.Buffer
			exit := make(chan int, 1)
			go func() {
				exit <- runCommand([]string{"fix", path}, terminal, terminal, &errOut, client.search)
			}()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			readPreviewOutput(t, ctx, master, "? Choose a recording")
			if tc.chooseRecording {
				if _, err := io.WriteString(master, "\r"); err != nil {
					t.Fatal(err)
				}
				readPreviewOutput(t, ctx, master, "Apply changes to this MP3")
			}
			if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, original) {
				t.Errorf("MP3 changed before approval: %v", err)
			}
			if _, err := os.Lstat(path + backupSuffix); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("backup exists before approval: %v", err)
			}
			if _, err := io.WriteString(master, tc.keys); err != nil {
				t.Fatal(err)
			}
			if tc.want != "" {
				readPreviewOutput(t, ctx, master, tc.want)
			}
			select {
			case code := <-exit:
				if code != tc.code {
					t.Fatalf("exit = %d, want %d; stderr = %q", code, tc.code, &errOut)
				}
			case <-ctx.Done():
				t.Fatal("fix did not exit")
			}
			if tc.name == "apply" {
				tags, err := readLookupTags(path)
				if err != nil {
					t.Fatal(err)
				}
				if title, artist := tags.searchTerms(); title != "Title" || artist != "Artist" {
					t.Errorf("edited tags = (%q, %q)", title, artist)
				}
				if backup, err := os.ReadFile(path + backupSuffix); err != nil || !bytes.Equal(backup, original) {
					t.Errorf("backup differs from original: %v", err)
				}
			} else {
				if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, original) {
					t.Errorf("MP3 changed without approval: %v", err)
				}
				if _, err := os.Lstat(path + backupSuffix); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("backup exists without approval: %v", err)
				}
			}
			assertNoFixStages(t, path)
		})
	}
}

func TestFixRequiresTerminal(t *testing.T) {
	path, _ := saveFixFixture(t)
	var out, errOut bytes.Buffer
	code := runCommand([]string{"fix", path}, strings.NewReader("\n"), &out, &errOut, func(_ context.Context, _, _ string) ([]recording, error) {
		t.Fatal("fix attempted a lookup without a terminal")
		return nil, nil
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "fix requires a terminal") {
		t.Errorf("exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
}
