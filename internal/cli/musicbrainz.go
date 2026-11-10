package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/palharez/l1ner/internal/ratelimit"
)

const (
	musicBrainzEndpoint = "https://musicbrainz.org/ws/2/recording"
	musicBrainzAgent    = "l1ner/devel (https://github.com/palharez/l1ner)"
	musicBrainzLimit    = 5
	musicBrainzTimeout  = 10 * time.Second
	maxSearchURL        = 4096
	maxSearchResponse   = 4 << 20
	musicBrainzInterval = time.Second
)

var errMusicBrainzCooldown = errors.New("MusicBrainz lookup is rate limited locally; wait a second and try again")

type recordingSearch func(ctx context.Context, title, artist string) ([]recording, error)

type musicBrainzClient struct {
	endpoint string
	rateFile string
	timeout  time.Duration
}

type recording struct {
	id, title, artist, disambiguation string
	lengthMS                          int64
}

func searchMusicBrainz(ctx context.Context, title, artist string) ([]recording, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("find MusicBrainz rate limit directory: %w", err)
	}
	client := musicBrainzClient{
		endpoint: musicBrainzEndpoint,
		rateFile: filepath.Join(cache, "l1ner", "musicbrainz-rate"),
		timeout:  musicBrainzTimeout,
	}
	return client.search(ctx, title, artist)
}

func (c musicBrainzClient) search(ctx context.Context, title, artist string) ([]recording, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	request, err := newRecordingSearchRequest(ctx, c.endpoint, title, artist)
	if err != nil {
		return nil, err
	}
	rateFile, err := ratelimit.Acquire(c.rateFile, "MusicBrainz", musicBrainzInterval)
	if errors.Is(err, ratelimit.ErrBusy) {
		return nil, errMusicBrainzCooldown
	}
	if err != nil {
		return nil, err
	}
	defer rateFile.Close()
	return requestRecordings(request, c.timeout)
}

func newRecordingSearchRequest(ctx context.Context, endpoint, title, artist string) (*http.Request, error) {
	if len(title)+len(artist) > maxSearchURL {
		return nil, errors.New("title and artist tags are too long for MusicBrainz lookup")
	}
	// Inside a Lucene quoted phrase, escape backslashes and quotes before URL encoding.
	escape := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	params := url.Values{
		"query": {`recording:"` + escape.Replace(title) + `" AND artist:"` + escape.Replace(artist) + `"`},
		"fmt":   {"json"},
		"limit": {fmt.Sprint(musicBrainzLimit)},
	}
	searchURL := endpoint + "?" + params.Encode()
	if len(searchURL) > maxSearchURL {
		return nil, errors.New("title and artist tags are too long for MusicBrainz lookup")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build MusicBrainz request: %w", err)
	}
	request.Header.Set("User-Agent", musicBrainzAgent)
	return request, nil
}

func requestRecordings(request *http.Request, timeout time.Duration) ([]recording, error) {
	client := http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		var requestError *url.Error
		if errors.As(err, &requestError) {
			err = requestError.Err // Keep the query out of the diagnostic.
		}
		return nil, fmt.Errorf("MusicBrainz request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("MusicBrainz is busy (HTTP %d); try again later", response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz returned HTTP %d", response.StatusCode)
	}
	return decodeRecordings(response.Body)
}

// paceBatchSearch is used sequentially by one batch. Leave a full interval
// after a request ends, since network latency can vary between songs.
func paceBatchSearch(search recordingSearch) recordingSearch {
	var next time.Time
	return func(ctx context.Context, title, artist string) ([]recording, error) {
		deadline := time.Now().Add(musicBrainzTimeout)
		for {
			if delay := time.Until(next); delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case <-timer.C:
				}
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			matches, err := search(ctx, title, artist)
			next = time.Now().Add(musicBrainzInterval)
			// A local reservation failure sent no request. Actual service
			// errors are reported for this song and are never retried here.
			if !errors.Is(err, errMusicBrainzCooldown) || next.After(deadline) {
				return matches, err
			}
		}
	}
}

func decodeRecordings(body io.Reader) ([]recording, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxSearchResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read MusicBrainz response: %w", err)
	}
	if len(data) > maxSearchResponse {
		return nil, errors.New("MusicBrainz response exceeds the size limit")
	}
	var result struct {
		Recordings []struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			Disambiguation string `json:"disambiguation"`
			LengthMS       int64  `json:"length"`
			ArtistCredit   []struct {
				Name       string `json:"name"`
				JoinPhrase string `json:"joinphrase"`
				Artist     struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"artist-credit"`
		} `json:"recordings"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode MusicBrainz response: %w", err)
	}
	if result.Recordings == nil {
		return nil, errors.New("MusicBrainz response is missing the recordings list")
	}
	limit := min(len(result.Recordings), musicBrainzLimit)
	matches := make([]recording, 0, limit)
	for _, item := range result.Recordings[:limit] {
		var credit strings.Builder
		for _, artist := range item.ArtistCredit {
			name := artist.Name
			if name == "" {
				name = artist.Artist.Name
			}
			if name == "" {
				return nil, errors.New("MusicBrainz response has an incomplete artist credit")
			}
			credit.WriteString(name)
			credit.WriteString(artist.JoinPhrase)
		}
		if item.ID == "" || item.Title == "" || credit.Len() == 0 {
			return nil, errors.New("MusicBrainz response has an incomplete recording")
		}
		matches = append(matches, recording{
			id: item.ID, title: item.Title, artist: credit.String(),
			disambiguation: item.Disambiguation, lengthMS: item.LengthMS,
		})
	}
	return matches, nil
}
