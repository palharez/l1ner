package identify

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func stubFPCalc(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "fpcalc"), []byte("#!/bin/sh\nset -eu\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func TestIdentifySendsFingerprintOnlyAndKeepsAmbiguity(t *testing.T) {
	stubFPCalc(t, `
test "$#" -eq 4
test "$1" = -json
test "$2" = -format
test "$3" = mp3
test -r "$4"
printf '%s\n' '{"duration":178.29,"fingerprint":"sample-fingerprint"}'
`)
	path := filepath.Join(t.TempDir(), "song without tags.MP3")
	original := []byte{0xff, 0xfb, 0x90, 0}
	if err := os.WriteFile(path, original, 0o400); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.UserAgent() != acoustIDAgent {
			t.Errorf("unexpected request %s %s, user agent %q", r.Method, r.URL, r.UserAgent())
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		want := map[string]string{
			"client": "test-app-key", "duration": "178", "fingerprint": "sample-fingerprint", "format": "json", "meta": "recordings",
		}
		if len(r.PostForm) != len(want) {
			t.Errorf("sent %d form fields, want %d", len(r.PostForm), len(want))
		}
		for name, value := range want {
			if !slices.Equal(r.PostForm[name], []string{value}) {
				t.Errorf("unexpected value for %s", name)
			}
		}
		io.WriteString(w, `{"status":"ok","results":[
			{"score":0.68,"recordings":[{"id":"live","title":"Song (Live)","artists":[{"name":"Artist"}]}]},
			{"score":0.96,"recordings":[
				{"id":"studio","title":"Song","artists":[{"name":"Artist"},{"name":"Guest"}]},
				{"id":"live"}
			]},
			{"score":0.99}
		]}`)
	}))
	defer server.Close()
	client := acoustIDClient{endpoint: server.URL, rateFile: filepath.Join(t.TempDir(), "acoustid-rate")}
	candidates, err := client.identify(t.Context(), path, "test-app-key")
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{
		{RecordingID: "live", Title: "Song (Live)", Artist: "Artist", Score: 0.96},
		{RecordingID: "studio", Title: "Song", Artist: "Artist, Guest", Score: 0.96},
	}
	if !slices.Equal(candidates, want) {
		t.Errorf("candidates = %+v, want %+v", candidates, want)
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(original, after) {
		t.Errorf("input changed: %v", err)
	}
}

func TestAcoustIDOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantError string
		status                int
	}{
		{"no matches", `{"status":"ok","results":[]}`, "", 200},
		{"unlinked match", `{"status":"ok","results":[{"score":0.9}]}`, "", 200},
		{"provider error", `{"status":"error","error":{"code":4,"message":"echoed test-app-key"}}`, "code 4", 200},
		{"malformed", `{"status":`, "invalid AcoustID JSON response", 200},
		{"missing results", `{"status":"ok"}`, "missing the results list", 200},
		{"busy", "", "AcoustID is busy (HTTP 503)", 503},
		{"redirect", "", "AcoustID returned HTTP 302", 302},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/lookup" {
					t.Errorf("request followed a redirect: %s", r.URL.Path)
				}
				if tc.status == 302 {
					w.Header().Set("Location", "/unexpected")
				}
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client := acoustIDClient{endpoint: server.URL + "/lookup", rateFile: filepath.Join(t.TempDir(), "rate")}
			got, err := client.lookup(t.Context(), fingerprint{text: "audio-fingerprint", duration: 178}, "test-app-key")
			if tc.wantError == "" {
				if err != nil || len(got) != 0 {
					t.Errorf("lookup = (%v, %v), want empty results", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) || strings.Contains(err.Error(), "test-app-key") {
				t.Errorf("lookup error = %v, want %q without the key", err, tc.wantError)
			}
		})
	}
}

func TestFPCalcFailures(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"missing tool", "", "fpcalc was not found"},
		{"decode failure", `printf 'invalid audio\n' >&2; exit 3`, "fpcalc failed: invalid audio"},
		{"malformed output", `printf 'not JSON\n'`, "decode fpcalc output"},
		{"empty fingerprint", `printf '%s\n' '{"duration":178.29,"fingerprint":""}'`, "no usable fingerprint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.script == "" {
				t.Setenv("PATH", t.TempDir())
			} else {
				stubFPCalc(t, tc.script)
			}
			path := filepath.Join(t.TempDir(), "song.mp3")
			if err := os.WriteFile(path, []byte{0xff, 0xfb, 0x90, 0}, 0o400); err != nil {
				t.Fatal(err)
			}
			if _, err := fingerprintMP3(t.Context(), path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("fingerprint error = %v, want %q", err, tc.want)
			}
		})
	}
}
