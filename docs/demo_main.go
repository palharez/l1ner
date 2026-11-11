//go:build ignore

// This executable is built explicitly by record-demo.sh for the VHS recording.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/palharez/l1ner/internal/cli"
)

func main() {
	http.DefaultTransport = demoCatalog{}
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// demoCatalog supplies the same responses on every run so the recording works offline.
type demoCatalog struct{}

func (demoCatalog) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method != http.MethodGet || request.URL.Host != "musicbrainz.org" {
		return nil, fmt.Errorf("unexpected request in demo: %s %s", request.Method, request.URL.Host)
	}

	var body string
	switch request.URL.Path {
	case "/ws/2/recording":
		body = `{"recordings":[{
			"id":"sample-recording",
			"title":"First Light",
			"artist-credit":[{"name":"Paper Satellites"}]
		}]}`
	case "/ws/2/release":
		body = `{"releases":[{
			"id":"sample-release",
			"title":"Night Signals",
			"date":"2024-09-20",
			"country":"US",
			"artist-credit":[{"name":"Paper Satellites"}],
			"media":[{"position":1,"format":"Digital Media","tracks":[
				{"position":1,"recording":{"id":"sample-recording"}}
			]}]
		}]}`
	default:
		return nil, fmt.Errorf("unexpected catalog path in demo: %s", request.URL.Path)
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
