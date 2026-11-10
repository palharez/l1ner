package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/big"

	"github.com/bogem/id3v2/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Decode the public field names independently of the production report types.
type decodedReport struct {
	Version int `json:"schema_version"`
	Files   []struct {
		Path     string `json:"path"`
		Status   string `json:"status"`
		Metadata []struct {
			ID          string  `json:"id"`
			Label       string  `json:"label"`
			Kind        string  `json:"kind"`
			Value       *string `json:"value"`
			Size        *int    `json:"size_bytes"`
			MIME        string  `json:"mime_type"`
			PictureType *byte   `json:"picture_type"`
			Description string  `json:"description"`
		} `json:"metadata"`
		Error string `json:"error"`
	} `json:"files"`
	ScanErrors []struct {
		Path    string `json:"path"`
		Message string `json:"message"`
	} `json:"scan_errors"`
	Summary struct {
		Inspected  int `json:"inspected"`
		Failed     int `json:"failed"`
		ScanErrors int `json:"scan_errors"`
	} `json:"summary"`
}

func decodeReport(t *testing.T, data []byte) decodedReport {
	t.Helper()
	var report decodedReport
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&report); err != nil {
		t.Fatalf("invalid JSON: %v; output %q", err, data)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected one JSON document; got %v", err)
	}
	if report.Version != 1 || report.Files == nil || report.ScanErrors == nil {
		t.Fatalf("missing version or non-null report arrays: %s", data)
	}
	for _, file := range report.Files {
		if file.Metadata == nil {
			t.Fatalf("null metadata array: %s", data)
		}
	}
	return report
}

func TestJSONSingleFile(t *testing.T) {
	for _, version := range []byte{3, 4} {
		path := save(t, fixture(version,
			frame(version, "TPE1", []byte{0, 'A'}),
			frame(version, "TIT2", append([]byte{1}, 0xff, 0xfe, 'O', 0, 'l', 0, 0xe1, 0, '\n', 0, 27, 0)),
			frame(version, "TPE1", []byte{0, 'B'}),
			frame(version, "TXXX", []byte{0, 'M', 'o', 'o', 'd', 0, 'C', 'a', 'l', 'm'}),
			frame(version, "APIC", append([]byte{0, 'i', 'm', 'a', 'g', 'e', '/', 'j', 'p', 'e', 'g', 0, 0, 0}, []byte("SECRET IMAGE")...)),
			frame(version, "XABC", []byte("SECRET UNKNOWN")),
		))
		before := inspectionSnapshot(t, path)
		var out, errOut bytes.Buffer
		if code := run([]string{"inspect", "--json", path}, &out, &errOut); code != 0 || errOut.Len() != 0 {
			t.Fatalf("code=%d stderr=%q", code, errOut.String())
		}
		report := decodeReport(t, out.Bytes())
		if len(report.Files) != 1 || report.Files[0].Path != path || report.Files[0].Status != "ok" || report.Summary.Inspected != 1 || report.Summary.Failed != 0 {
			t.Fatalf("wrong result: %s", &out)
		}
		metadata := report.Files[0].Metadata
		if len(metadata) != 6 {
			t.Fatalf("metadata count=%d; want 6", len(metadata))
		}
		if metadata[0].ID != "APIC" || metadata[0].Kind != "binary" || metadata[0].MIME != "image/jpeg" || metadata[0].Size == nil || *metadata[0].Size != 12 || metadata[0].PictureType == nil || *metadata[0].PictureType != 0 {
			t.Errorf("wrong artwork summary: %+v", metadata[0])
		}
		for i, want := range []struct{ id, value string }{{"TIT2", "Olá\n\x1b"}, {"TPE1", "A"}, {"TPE1", "B"}, {"TXXX", "Calm"}} {
			got := metadata[i+1]
			if got.ID != want.id || got.Value == nil || *got.Value != want.value {
				t.Errorf("metadata[%d]=%+v; want %+v", i+1, got, want)
			}
		}
		if metadata[4].Description != "Mood" || metadata[5].Kind != "unknown" || metadata[5].Size == nil || *metadata[5].Size != 14 {
			t.Errorf("custom/unknown summaries: %s", &out)
		}
		if strings.Contains(out.String(), "SECRET") || bytes.Contains(out.Bytes(), []byte{27}) {
			t.Error("raw binary data or terminal escape leaked")
		}
		if after := inspectionSnapshot(t, path); after != before {
			t.Error("input changed")
		}
	}
}

func TestJSONDirectory(t *testing.T) {
	root := t.TempDir()
	good := batchFile(t, root, "a.MP3", fixture(4))
	bad := batchFile(t, root, "b.mp3", fixture(2))
	nested := batchFile(t, root, "album/c.mp3", fixture(3))
	batchFile(t, root, "notes.txt", []byte("ignored"))
	for _, recursive := range []bool{false, true} {
		args := []string{"inspect", "--json"}
		wantCount := 2
		if recursive {
			args = append(args, "--recursive")
			wantCount++
		}
		args = append(args, root)
		before := map[string]fileSnapshot{}
		for _, path := range []string{good, bad, nested} {
			before[path] = inspectionSnapshot(t, path)
		}
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 1 || errOut.Len() != 0 {
			t.Fatalf("code=%d stderr=%q", code, errOut.String())
		}
		report := decodeReport(t, out.Bytes())
		if len(report.Files) != wantCount || report.Summary.Inspected != wantCount-1 || report.Summary.Failed != 1 || report.Summary.ScanErrors != 0 {
			t.Fatalf("wrong summary: %s", &out)
		}
		var failed bool
		for _, file := range report.Files {
			if file.Path == bad {
				failed = true
				if file.Status != "error" || file.Error != "unsupported tag version: ID3v2.2" || len(file.Metadata) != 0 {
					t.Errorf("wrong failed result: %+v", file)
				}
			}
			if file.Path != bad && (file.Status != "ok" || len(file.Metadata) != 0) {
				t.Errorf("wrong untagged result: %+v", file)
			}
		}
		if !failed {
			t.Error("failed file omitted")
		}
		for path, snapshot := range before {
			if inspectionSnapshot(t, path) != snapshot {
				t.Errorf("input changed: %s", path)
			}
		}
	}
}

func TestJSONEmptyAndMissingInputs(t *testing.T) {
	for _, tt := range []struct {
		name, path          string
		code, files, failed int
	}{
		{"empty directory", t.TempDir(), 0, 0, 0},
		{"no tags", save(t, fixture(3)), 0, 1, 0},
		{"missing file", filepath.Join(t.TempDir(), "missing.mp3"), 1, 1, 1},
		{"malformed file", save(t, fixture(3)[:5]), 1, 1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run([]string{"inspect", "--json", tt.path}, &out, &errOut); code != tt.code || errOut.Len() != 0 {
				t.Fatalf("code=%d stderr=%q", code, errOut.String())
			}
			report := decodeReport(t, out.Bytes())
			if len(report.Files) != tt.files || report.Summary.Failed != tt.failed {
				t.Fatalf("unexpected report: %s", &out)
			}
			if tt.failed > 0 && report.Files[0].Error == "" {
				t.Error("missing error message")
			}
		})
	}
}

func TestJSONScanErrors(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"inspect", "--json", link}, &out, &errOut); code != 1 || errOut.Len() != 0 {
		t.Fatalf("code=%d stderr=%s", code, &errOut)
	}
	report := decodeReport(t, out.Bytes())
	if len(report.Files) != 0 || len(report.ScanErrors) != 1 || report.ScanErrors[0].Path != link || report.Summary.ScanErrors != 1 {
		t.Fatalf("wrong scan errors: %s", &out)
	}
}

func TestJSONUnreadableDirectoryContinues(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read directories without permission")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	good := batchFile(t, root, "z.mp3", fixture(3))
	var out, errOut bytes.Buffer
	if code := run([]string{"inspect", "--json", "--recursive", root}, &out, &errOut); code != 1 || errOut.Len() != 0 {
		t.Fatalf("code=%d stderr=%s", code, &errOut)
	}
	report := decodeReport(t, out.Bytes())
	if len(report.ScanErrors) != 1 || report.ScanErrors[0].Path != locked || report.ScanErrors[0].Message == "" || len(report.Files) != 1 || report.Files[0].Path != good || report.Summary.Inspected != 1 || report.Summary.ScanErrors != 1 {
		t.Fatalf("unexpected report: %s", &out)
	}
}

func TestJSONOutputFailure(t *testing.T) {
	for _, path := range []string{save(t, fixture(3)), t.TempDir()} {
		var errOut bytes.Buffer
		if code := run([]string{"inspect", "--json", path}, failingWriter{err: errors.New("closed")}, &errOut); code != 1 || errOut.String() != "Error: closed\n" {
			t.Errorf("code=%d stderr=%q", code, errOut.String())
		}
	}
}

func TestJSONInvalidArguments(t *testing.T) {
	path := save(t, fixture(3))
	for _, args := range [][]string{{"inspect", "--json"}, {"inspect", "--json", "--recursive", path}, {"inspect", "--json", "--unknown", path}} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 2 || out.Len() != 0 || errOut.Len() == 0 {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, out.String(), errOut.String())
		}
	}
}

func TestJSONMetadataDetails(t *testing.T) {
	count := new(big.Int).Lsh(big.NewInt(1), 64)
	tests := []struct {
		name string
		item entry
		want map[string]any
	}{
		{"empty text", entry{id: "TIT2", frame: id3v2.TextFrame{}}, map[string]any{"kind": "text", "value": ""}},
		{"comment", entry{id: "COMM", frame: id3v2.CommentFrame{Language: "eng", Description: "Notes", Text: "Live"}}, map[string]any{"language": "eng", "description": "Notes", "value": "Live"}},
		{"lyrics", entry{id: "USLT", frame: id3v2.UnsynchronisedLyricsFrame{Language: "por", ContentDescriptor: "Original", Lyrics: "Olá\nmundo"}}, map[string]any{"language": "por", "description": "Original", "value": "Olá\nmundo"}},
		{"empty identifier", entry{id: "UFID", frame: id3v2.UFIDFrame{OwnerIdentifier: "example.org"}}, map[string]any{"kind": "binary", "owner": "example.org", "size_bytes": float64(0)}},
		{"rating and large counter", entry{id: "POPM", frame: id3v2.PopularimeterFrame{Email: "listener", Counter: count}}, map[string]any{"kind": "rating", "owner": "listener", "rating": float64(0), "play_count": "18446744073709551616"}},
		{"URL", entry{id: "WOAR", frame: id3v2.UnknownFrame{Body: []byte("https://example.org/artist")}}, map[string]any{"kind": "text", "value": "https://example.org/artist"}},
		{"unknown empty payload", entry{id: "WXXX", frame: id3v2.UnknownFrame{}}, map[string]any{"kind": "unknown", "size_bytes": float64(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(frameJSON(tt.item))
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			for key, want := range tt.want {
				if got, present := fields[key]; !present || got != want {
					t.Errorf("%s = %v; want %v (present=%v)", key, got, want, present)
				}
			}
			if kind := fields["kind"]; kind == "binary" || kind == "unknown" {
				if _, present := fields["value"]; present {
					t.Error("binary data has a value field")
				}
			}
		})
	}
}
