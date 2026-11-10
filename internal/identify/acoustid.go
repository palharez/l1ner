// Package identify finds recording candidates from a locally generated audio fingerprint.
package identify

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/palharez/l1ner/internal/ratelimit"
)

const (
	acoustIDEndpoint = "https://api.acoustid.org/v2/lookup"
	acoustIDAgent    = "l1ner/devel (https://github.com/palharez/l1ner)"
	maxResponse      = 4 << 20
)

// Candidate is a MusicBrainz recording linked to an AcoustID fingerprint match.
type Candidate struct {
	RecordingID, Title, Artist string
	Score                      float64
}

// Search fingerprints the MP3 locally and sends the fingerprint and duration to AcoustID.
func Search(ctx context.Context, path, applicationKey string) ([]Candidate, error) {
	client, err := localAcoustIDClient()
	if err != nil {
		return nil, err
	}
	return client.identify(ctx, path, applicationKey)
}

// SearchFile fingerprints the open file so replacing its path cannot change the audio being reviewed.
func SearchFile(ctx context.Context, file *os.File, applicationKey string) ([]Candidate, error) {
	client, err := localAcoustIDClient()
	if err != nil {
		return nil, err
	}
	sample, err := runFPCalc(ctx, "/proc/self/fd/3", file)
	if err != nil {
		return nil, err
	}
	return client.lookup(ctx, sample, applicationKey)
}

func localAcoustIDClient() (acoustIDClient, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return acoustIDClient{}, fmt.Errorf("find AcoustID rate limit directory: %w", err)
	}
	return acoustIDClient{endpoint: acoustIDEndpoint, rateFile: filepath.Join(cache, "l1ner", "acoustid-rate")}, nil
}

type acoustIDClient struct {
	endpoint string
	rateFile string
}

func (client acoustIDClient) identify(ctx context.Context, path, applicationKey string) ([]Candidate, error) {
	sample, err := fingerprintMP3(ctx, path)
	if err != nil {
		return nil, err
	}
	return client.lookup(ctx, sample, applicationKey)
}

func (client acoustIDClient) lookup(ctx context.Context, sample fingerprint, applicationKey string) ([]Candidate, error) {
	fields := url.Values{
		"client":      {applicationKey},
		"duration":    {strconv.Itoa(sample.duration)},
		"fingerprint": {sample.text},
		"format":      {"json"},
		"meta":        {"recordings"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, strings.NewReader(fields.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build AcoustID request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", acoustIDAgent)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rateFile, err := ratelimit.Acquire(client.rateFile, "AcoustID", time.Second)
	if errors.Is(err, ratelimit.ErrBusy) {
		return nil, errors.New("AcoustID lookup is rate limited locally; wait a second and try again")
	}
	if err != nil {
		return nil, err
	}
	defer rateFile.Close()
	return requestCandidates(request)
}

func requestCandidates(request *http.Request) ([]Candidate, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("AcoustID request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("AcoustID is busy (HTTP %d); try again later", response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AcoustID returned HTTP %d", response.StatusCode)
	}
	return decodeCandidates(response.Body)
}

type fingerprintMatch struct {
	Score      float64 `json:"score"`
	Recordings []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"recordings"`
}

func decodeCandidates(body io.Reader) ([]Candidate, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read AcoustID response: %w", err)
	}
	if len(data) > maxResponse {
		return nil, errors.New("AcoustID response exceeded the size limit")
	}
	var output struct {
		Status  string             `json:"status"`
		Results []fingerprintMatch `json:"results"`
		Error   struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, errors.New("invalid AcoustID JSON response")
	}
	if output.Status == "error" {
		return nil, fmt.Errorf("AcoustID rejected the lookup (code %d)", output.Error.Code)
	}
	if output.Status != "ok" || output.Results == nil {
		return nil, errors.New("AcoustID response is missing the results list or status")
	}
	return collectCandidates(output.Results)
}

func collectCandidates(results []fingerprintMatch) ([]Candidate, error) {
	var candidates []Candidate
	byID := make(map[string]int)
	for _, match := range results {
		if match.Score < 0 || match.Score > 1 {
			return nil, errors.New("AcoustID response contains an invalid score")
		}
		for _, recording := range match.Recordings {
			if recording.ID == "" {
				continue
			}
			var names []string
			for _, artist := range recording.Artists {
				if artist.Name != "" {
					names = append(names, artist.Name)
				}
			}
			credit := strings.Join(names, ", ")
			if index, ok := byID[recording.ID]; ok {
				candidate := &candidates[index]
				candidate.Score = max(candidate.Score, match.Score)
				if candidate.Title == "" {
					candidate.Title = recording.Title
				}
				if candidate.Artist == "" {
					candidate.Artist = credit
				}
				continue
			}
			byID[recording.ID] = len(candidates)
			candidates = append(candidates, Candidate{RecordingID: recording.ID, Title: recording.Title, Artist: credit, Score: match.Score})
		}
	}
	slices.SortStableFunc(candidates, func(a, b Candidate) int { return cmp.Compare(b.Score, a.Score) })
	return candidates, nil
}
