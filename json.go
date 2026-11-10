package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/bogem/id3v2/v2"
)

type jsonReport struct {
	SchemaVersion int             `json:"schema_version"`
	Files         []jsonFile      `json:"files"`
	ScanErrors    []jsonScanError `json:"scan_errors"`
	Summary       jsonSummary     `json:"summary"`
}

type jsonFile struct {
	Path     string         `json:"path"`
	Status   string         `json:"status"`
	Metadata []jsonMetadata `json:"metadata"`
	Error    string         `json:"error,omitempty"`
}

type jsonScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type jsonSummary struct {
	Inspected  int `json:"inspected"`
	Failed     int `json:"failed"`
	ScanErrors int `json:"scan_errors"`
}

type jsonMetadata struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Kind        string  `json:"kind"`
	Value       *string `json:"value,omitempty"`
	SizeBytes   *int    `json:"size_bytes,omitempty"`
	MIMEType    string  `json:"mime_type,omitempty"`
	PictureType *byte   `json:"picture_type,omitempty"`
	Description string  `json:"description,omitempty"`
	Language    string  `json:"language,omitempty"`
	Owner       string  `json:"owner,omitempty"`
	Rating      *uint8  `json:"rating,omitempty"`
	PlayCount   string  `json:"play_count,omitempty"`
}

func runJSONInspection(options inspectOptions, out, errOut io.Writer) int {
	report := jsonReport{SchemaVersion: 1, Files: []jsonFile{}, ScanErrors: []jsonScanError{}}
	info, err := os.Stat(options.path)
	switch {
	case err != nil:
		report.Files = append(report.Files, jsonFile{
			Path: options.path, Status: "error", Metadata: []jsonMetadata{}, Error: err.Error(),
		})
		report.Summary.Failed++
	case info.IsDir():
		report = inspectJSONDirectory(options)
	case options.recursive:
		reportError(errOut, errors.New("--recursive requires a directory"))
		return 2
	default:
		report.addFile(options.path)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		reportError(errOut, err)
		return 1
	}
	if report.Summary.Failed > 0 || report.Summary.ScanErrors > 0 {
		return 1
	}
	return 0
}

func inspectJSONDirectory(options inspectOptions) jsonReport {
	report := jsonReport{SchemaVersion: 1, Files: []jsonFile{}, ScanErrors: []jsonScanError{}}
	scan := scanDirectory(options.path, options.recursive)
	for _, err := range scan.errors {
		path := options.path
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			path = pathErr.Path
		}
		report.ScanErrors = append(report.ScanErrors, jsonScanError{Path: path, Message: err.Error()})
	}
	report.Summary.ScanErrors = len(report.ScanErrors)
	for _, path := range scan.paths {
		report.addFile(path)
	}
	return report
}

func (report *jsonReport) addFile(path string) {
	file := jsonFile{Path: path, Status: "ok", Metadata: []jsonMetadata{}}
	entries, err := inspect(path)
	if err != nil {
		file.Status, file.Error = "error", err.Error()
		report.Summary.Failed++
	} else {
		file.Metadata = metadataJSON(entries)
		report.Summary.Inspected++
	}
	report.Files = append(report.Files, file)
}

func metadataJSON(entries []entry) []jsonMetadata {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	metadata := make([]jsonMetadata, 0, len(entries))
	for _, item := range entries {
		metadata = append(metadata, frameJSON(item))
	}
	return metadata
}

func frameJSON(item entry) jsonMetadata {
	label, _ := describe(item)
	metadata := jsonMetadata{ID: item.id, Label: label, Kind: "text"}
	switch frame := item.frame.(type) {
	case id3v2.TextFrame:
		metadata.Value = &frame.Text
	case id3v2.UserDefinedTextFrame:
		metadata.Description, metadata.Value = frame.Description, &frame.Value
	case id3v2.CommentFrame:
		metadata.Language, metadata.Description, metadata.Value = frame.Language, frame.Description, &frame.Text
	case id3v2.UnsynchronisedLyricsFrame:
		metadata.Language, metadata.Description, metadata.Value = frame.Language, frame.ContentDescriptor, &frame.Lyrics
	case id3v2.PictureFrame:
		metadata.Kind, metadata.MIMEType, metadata.PictureType = "binary", frame.MimeType, &frame.PictureType
		size := len(frame.Picture)
		metadata.Description, metadata.SizeBytes = frame.Description, &size
	case id3v2.UFIDFrame:
		size := len(frame.Identifier)
		metadata.Kind, metadata.Owner, metadata.SizeBytes = "binary", frame.OwnerIdentifier, &size
	case id3v2.PopularimeterFrame:
		metadata.Kind, metadata.Owner, metadata.Rating = "rating", frame.Email, &frame.Rating
		if frame.Counter != nil {
			metadata.PlayCount = frame.Counter.String()
		}
	case id3v2.UnknownFrame:
		size := len(frame.Body)
		metadata.Kind, metadata.SizeBytes = "unknown", &size
		if strings.HasPrefix(item.id, "W") && item.id != "WXXX" {
			value := string(frame.Body)
			metadata.Kind, metadata.Value, metadata.SizeBytes = "text", &value, nil
		}
	default:
		size := item.frame.Size()
		metadata.Kind, metadata.SizeBytes = "unknown", &size
	}
	return metadata
}
