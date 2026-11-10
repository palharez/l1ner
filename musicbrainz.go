package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	musicBrainzEndpoint = "https://musicbrainz.org/ws/2/recording"
	musicBrainzAgent    = "l1ner/devel (https://github.com/palharez/l1ner)"
	musicBrainzLimit    = 5
	musicBrainzTimeout  = 10 * time.Second
	maxSearchURL        = 4096
	maxSearchResponse   = 4 << 20
)

type musicBrainzClient struct {
	endpoint string
	rateFile string
	timeout  time.Duration
}

type recording struct {
	id, title, artist string
}

func (c musicBrainzClient) search(title, artist string) ([]recording, error) {
	request, err := newRecordingSearchRequest(c.endpoint, title, artist)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(c.rateFile), 0o700); err != nil {
		return nil, fmt.Errorf("create MusicBrainz rate limit directory: %w", err)
	}
	rateFile, err := os.OpenFile(c.rateFile, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open MusicBrainz rate limit file: %w", err)
	}
	defer rateFile.Close()
	if err := reserveMusicBrainzRequest(rateFile); err != nil {
		return nil, err
	}
	return requestRecordings(request, c.timeout)
}

func newRecordingSearchRequest(endpoint, title, artist string) (*http.Request, error) {
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
	request, err := http.NewRequest(http.MethodGet, searchURL, nil)
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

func reserveMusicBrainzRequest(file *os.File) error {
	const waitMessage = "MusicBrainz lookup is rate limited locally; wait a second and try again"
	// Each invocation is a new process. Hold the shared lock until the request ends.
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return errors.New(waitMessage)
		}
		return fmt.Errorf("lock MusicBrainz rate limit file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat MusicBrainz rate limit file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("MusicBrainz rate limit file is not a regular file")
	}
	if info.Size() != 0 && time.Since(info.ModTime()) < time.Second {
		return errors.New(waitMessage)
	}
	// The single byte distinguishes a new file. Its modification time records the last request.
	if _, err := file.WriteAt([]byte{1}, 0); err != nil {
		return fmt.Errorf("update MusicBrainz rate limit file: %w", err)
	}
	return nil
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
			ID           string `json:"id"`
			Title        string `json:"title"`
			ArtistCredit []struct {
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
		matches = append(matches, recording{item.ID, item.Title, credit.String()})
	}
	return matches, nil
}
