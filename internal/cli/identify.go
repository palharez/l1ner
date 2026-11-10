package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/palharez/l1ner/internal/identify"
)

const identifySyntax = "l1ner identify <file.mp3>"
const acoustIDRegistrationURL = "https://acoustid.org/new-application"

const identifyHelp = "Usage: " + identifySyntax + "\n\n" +
	"Optionally identify an MP3 by sound. The other commands work without AcoustID setup.\n" +
	"Install Chromaprint's fpcalc utility on PATH. Set L1NER_ACOUSTID_KEY to the\n" +
	"application API key from " + acoustIDRegistrationURL + ".\n" +
	"The fingerprint is calculated locally. AcoustID receives the fingerprint and duration."

func runIdentify(args []string, out, errOut io.Writer, search func(context.Context, string, string) ([]identify.Candidate, error)) int {
	path, err := parseSongPath("identify", args)
	if errors.Is(err, flag.ErrHelp) {
		_, err := fmt.Fprintln(out, identifyHelp)
		if err != nil {
			reportError(errOut, err)
			return 1
		}
		return 0
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, "Usage: "+identifySyntax)
		return 2
	}
	key := strings.TrimSpace(os.Getenv("L1NER_ACOUSTID_KEY"))
	if key == "" {
		reportError(errOut, errors.New("set L1NER_ACOUSTID_KEY to the application API key from "+acoustIDRegistrationURL))
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	candidates, err := search(ctx, path, key)
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return 130
	}
	if err == nil {
		err = displayIdentification(out, path, candidates)
	}
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	return 0
}

func displayIdentification(out io.Writer, path string, candidates []identify.Candidate) error {
	var output strings.Builder
	fmt.Fprintf(&output, "File: %s\n\n", safe(path))
	if len(candidates) == 0 {
		output.WriteString("No MusicBrainz recording candidates found.\n")
	} else {
		output.WriteString("MusicBrainz recording candidates from AcoustID:\n")
		for i, candidate := range candidates {
			title, artist := candidate.Title, candidate.Artist
			if title == "" {
				title = "Unknown title"
			}
			if artist == "" {
				artist = "Unknown artist"
			}
			fmt.Fprintf(&output, "%d. %s — %s\n   Recording ID: %s\n   AcoustID score: %.4f\n",
				i+1, safe(title), safe(artist), safe(candidate.RecordingID), candidate.Score)
		}
	}
	_, err := io.WriteString(out, output.String())
	return err
}
