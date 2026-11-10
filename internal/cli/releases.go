package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const releaseEndpoint = "https://musicbrainz.org/ws/2/release"
const releaseLimit = 20

type releaseSearch func(context.Context, string) ([]releaseTrack, error)

type releaseTrack struct {
	id, album, albumArtist, date, country, format string
	disc, track                                   int
}

func searchMusicBrainzReleases(ctx context.Context, recordingID string) ([]releaseTrack, error) {
	client, err := localMusicBrainzClient(releaseEndpoint)
	if err != nil {
		return nil, err
	}
	return client.releases(ctx, recordingID)
}

func (c musicBrainzClient) releases(ctx context.Context, recordingID string) ([]releaseTrack, error) {
	if recordingID == "" || len(recordingID) > 64 {
		return nil, errors.New("invalid MusicBrainz recording ID for release lookup")
	}
	params := url.Values{
		"recording": {recordingID},
		"inc":       {"recordings+artist-credits"},
		"fmt":       {"json"},
		"limit":     {strconv.Itoa(releaseLimit)},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build MusicBrainz release request: %w", err)
	}
	request.Header.Set("User-Agent", musicBrainzAgent)
	wait, cancel := context.WithTimeout(ctx, musicBrainzTimeout)
	defer cancel()
	for wait.Err() == nil {
		slot, err := c.acquireSlot()
		if err == nil {
			defer slot.Close()
			data, err := fetchMusicBrainz(request, c.timeout)
			if err != nil {
				return nil, err
			}
			return decodeReleaseTracks(data, recordingID)
		}
		if !errors.Is(err, errMusicBrainzCooldown) {
			return nil, err
		}
		select {
		case <-wait.Done():
		case <-time.After(musicBrainzInterval):
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errMusicBrainzCooldown
}

func decodeReleaseTracks(data []byte, recordingID string) ([]releaseTrack, error) {
	var result struct {
		Releases []struct {
			ID           string         `json:"id"`
			Title        string         `json:"title"`
			Date         string         `json:"date"`
			Country      string         `json:"country"`
			ArtistCredit []artistCredit `json:"artist-credit"`
			Media        []struct {
				Position int    `json:"position"`
				Format   string `json:"format"`
				Tracks   []struct {
					Position  int `json:"position"`
					Recording struct {
						ID string `json:"id"`
					} `json:"recording"`
				} `json:"tracks"`
			} `json:"media"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode MusicBrainz releases: %w", err)
	}
	if result.Releases == nil {
		return nil, errors.New("MusicBrainz response is missing the releases list")
	}
	var matches []releaseTrack
	for _, release := range result.Releases[:min(len(result.Releases), releaseLimit)] {
		if release.ID == "" || strings.TrimSpace(release.Title) == "" {
			continue
		}
		artist, _ := formatArtistCredit(release.ArtistCredit)
		for _, medium := range release.Media {
			for _, track := range medium.Tracks {
				if track.Recording.ID == recordingID && medium.Position > 0 && track.Position > 0 {
					matches = append(matches, releaseTrack{
						id: release.ID, album: release.Title, albumArtist: artist, date: release.Date, country: release.Country,
						format: medium.Format, disc: medium.Position, track: track.Position,
					})
				}
			}
		}
	}
	return matches, nil
}
