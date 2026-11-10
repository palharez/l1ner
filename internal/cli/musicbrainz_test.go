package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/palharez/l1ner/internal/ratelimit"
)

func TestBatchSearchPacesRequestsAndCancelsWait(t *testing.T) {
	arrivals := make(chan time.Time, 2)
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		arrivals <- time.Now()
		io.WriteString(w, `{"recordings":[]}`)
	})
	search := paceRecordingSearch(client.search)
	for _, title := range []string{"First", "Second"} {
		if _, err := search(t.Context(), title, "Artist"); err != nil {
			t.Fatal(err)
		}
	}
	first, second := <-arrivals, <-arrivals
	if gap := second.Sub(first); gap < musicBrainzInterval {
		t.Errorf("requests spaced %v apart, want at least %v", gap, musicBrainzInterval)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := search(ctx, "Third", "Artist"); !errors.Is(err, context.Canceled) {
		t.Errorf("search error = %v, want context canceled", err)
	}
	if len(arrivals) != 0 {
		t.Error("canceled search reached the server")
	}
}

func TestBatchSearchWaitsForLocalReservation(t *testing.T) {
	var requests atomic.Int32
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		io.WriteString(w, `{"recordings":[]}`)
	})
	if _, err := client.search(t.Context(), "Earlier", "Artist"); err != nil {
		t.Fatal(err)
	}
	if _, err := paceRecordingSearch(client.search)(t.Context(), "First batch song", "Artist"); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}

func testMusicBrainz(t *testing.T, handler http.HandlerFunc) musicBrainzClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return musicBrainzClient{
		endpoint: server.URL + "/ws/2/recording",
		rateFile: filepath.Join(t.TempDir(), "l1ner", "musicbrainz-rate"),
		timeout:  musicBrainzTimeout,
	}
}

func TestMusicBrainzSearch(t *testing.T) {
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ws/2/recording" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.UserAgent(); got != "l1ner/devel (https://github.com/palharez/l1ner)" {
			t.Errorf("User-Agent = %q", got)
		}
		params := r.URL.Query()
		wantQuery := `recording:"Café \"Live\" \\ Mix" AND artist:"A+B & C"`
		if len(params) != 3 || params.Get("query") != wantQuery || params.Get("fmt") != "json" || params.Get("limit") != "5" {
			t.Errorf("search parameters = %v", params)
		}
		io.WriteString(w, `{"recordings":[`)
		for i := range 6 {
			if i > 0 {
				io.WriteString(w, ",")
			}
			fmt.Fprintf(w, `{"id":"id-%d","title":"Café","artist-credit":[{"name":"A+B","joinphrase":" & ","artist":{"name":"Other"}},{"artist":{"name":"C"}}]}`, i)
		}
		io.WriteString(w, `]}`)
	})
	matches, err := client.search(t.Context(), `Café "Live" \ Mix`, "A+B & C")
	if err != nil || len(matches) != 5 {
		t.Fatalf("search = (%v, %v), want five recordings", matches, err)
	}
	for i, got := range matches {
		want := recording{id: fmt.Sprintf("id-%d", i), title: "Café", artist: "A+B & C"}
		if got != want {
			t.Errorf("recording %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestMusicBrainzSearchOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		error  string
	}{
		{"empty results", 200, `{"count":0,"recordings":[]}`, ""},
		{"malformed JSON", 200, `{"recordings":[`, "decode MusicBrainz response"},
		{"missing recordings", 200, `{}`, "missing the recordings list"},
		{"throttled", 429, ``, "MusicBrainz is busy (HTTP 429)"},
		{"unavailable", 503, ``, "MusicBrainz is busy (HTTP 503)"},
		{"redirect", 302, ``, "MusicBrainz returned HTTP 302"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/ws/2/recording" {
					t.Errorf("unexpected request to %s", r.URL.Path)
				}
				if tc.status == 302 {
					w.Header().Set("Location", "/unexpected")
				}
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			matches, err := client.search(t.Context(), "Title", "Artist")
			if tc.error == "" {
				if err != nil || len(matches) != 0 {
					t.Errorf("search = (%v, %v), want empty results", matches, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.error) || len(matches) != 0 {
				t.Errorf("search = (%v, %v), want error containing %q", matches, err, tc.error)
			}
		})
	}
}

func TestMusicBrainzTimeout(t *testing.T) {
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	client.timeout = 50 * time.Millisecond
	_, err := client.search(t.Context(), "Private title", "Artist")
	if err == nil || !strings.Contains(err.Error(), "Timeout") || strings.Contains(err.Error(), "Private") {
		t.Errorf("search error = %v, want timeout without the query", err)
	}
}

func TestMusicBrainzRateLimit(t *testing.T) {
	var requests atomic.Int32
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		io.WriteString(w, `{"recordings":[]}`)
	})
	file, err := ratelimit.Acquire(client.rateFile, "MusicBrainz", musicBrainzInterval)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	if _, err := client.search(t.Context(), "Title", "Artist"); err == nil || !strings.Contains(err.Error(), "rate limited locally") {
		t.Errorf("search with held lock = %v, want local rate limit error", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(client.rateFile, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := client.search(t.Context(), "Title", "Artist"); err == nil || !strings.Contains(err.Error(), "rate limited locally") {
		t.Errorf("search after recent request = %v, want local rate limit error", err)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("requests during cooldown = %d, want 0", got)
	}
	earlier := now.Add(-2 * time.Second)
	if err := os.Chtimes(client.rateFile, earlier, earlier); err != nil {
		t.Fatal(err)
	}
	if _, err := client.search(t.Context(), "Title", "Artist"); err != nil {
		t.Errorf("search after cooldown = %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("requests after cooldown = %d, want 1", got)
	}
}
