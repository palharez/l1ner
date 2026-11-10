package cli

import (
	"io"
	"net/http"
	"slices"
	"testing"
)

func TestBrowseReleaseTracks(t *testing.T) {
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if r.Method != http.MethodGet || query.Get("recording") != "chosen" ||
			query.Get("inc") != "recordings+artist-credits" || query.Get("limit") != "20" ||
			query.Get("fmt") != "json" || r.UserAgent() != musicBrainzAgent {
			t.Errorf("unexpected release request %s %s", r.Method, r.URL)
		}
		io.WriteString(w, `{"releases":[
			{"id":"album","title":"Album","date":"1960","artist-credit":[{"name":"Artist","joinphrase":" & "},{"artist":{"name":"Guest"}}],
			 "media":[{"position":1,"format":"CD","tracks":[{"position":1,"recording":{"id":"other"}},{"position":6,"recording":{"id":"chosen"}}]}]},
			{"id":"compilation","title":"Compilation","date":"2000","country":"US",
			 "media":[{"position":2,"tracks":[{"position":9,"recording":{"id":"chosen"}}]}]},
			{"id":"unrelated","title":"Unrelated","media":[{"position":1,"tracks":[{"position":4,"recording":{"id":"other"}}]}]}
		]}`)
	})
	got, err := client.releases(t.Context(), "chosen")
	want := []releaseTrack{
		{id: "album", album: "Album", date: "1960", albumArtist: "Artist & Guest", format: "CD", disc: 1, track: 6},
		{id: "compilation", album: "Compilation", date: "2000", country: "US", disc: 2, track: 9},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("releases = %+v, error = %v, want %+v", got, err, want)
	}
}
