package cli

import (
	"strings"
	"testing"
)

func TestMenuFitsNarrowTerminal(t *testing.T) {
	const width, height = 42, 8
	options := []menuOption{
		{label: "A title above the viewport"},
		{label: "A recording with a very long title and artist"},
		{
			label: "A recording with a long title — Example Artist",
			details: []string{
				"Recording ID: 232265e2-7983-461a-918a-be86d0b87a9c",
				"Version: live, 1959-11: Kresge Auditorium, Cambridge, Massachusetts",
			},
		},
		{label: "Skip song"},
		{label: "Quit"},
	}
	lines := menuLines("Choose a recording", options, 2, width, height, false)
	if len(lines) >= height {
		t.Errorf("menu draws %d lines in a %d-line terminal", len(lines), height)
	}
	for _, line := range lines {
		if fitMenuLine(line, width) != line || strings.Contains(line, "\x1b") {
			t.Errorf("line wraps or contains styling: %q", line)
		}
	}
	text := strings.Join(lines, "\n")
	for _, want := range []string{"❯ A recording with a long title", "232265e2-7983-461a-918a-be86d0b87a9c", "Version: live"} {
		if !strings.Contains(text, want) {
			t.Errorf("menu is missing %q: %s", want, text)
		}
	}
}
