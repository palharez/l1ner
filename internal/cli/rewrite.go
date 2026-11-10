package cli

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// rewriteTag changes only the approved text frames. A whole-tag library save
// would lose repeated frames and reserialize frames that the user did not edit.
func rewriteTag(header, body []byte, proposal songProposal) ([]byte, error) {
	if header == nil {
		header = []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0}
	}
	if len(header) != tagHeaderSize || (header[3] != 3 && header[3] != 4) || header[4] != 0 || header[5] != 0 {
		return nil, errors.New("fix requires an ID3v2.3.0 or ID3v2.4.0 tag")
	}
	version := header[3]
	replacements, err := approvedTextFrames(version, proposal)
	if err != nil {
		return nil, err
	}
	tag := append(make([]byte, 0, len(header)+len(body)), header...)
	var offsetFrame string
	found := make(map[string]bool)
	for rest := body; len(rest) > 0; {
		if rest[0] == 0 {
			if len(bytes.Trim(rest, "\x00")) != 0 {
				return nil, errors.New("malformed ID3 padding")
			}
			break
		}
		frame, err := parseFrameHeader(rest, version)
		if err != nil {
			return nil, err
		}
		size := frameHeaderSize + frame.bodySize
		original := rest[:size]
		rest = rest[size:]
		if original[8] != 0 || frame.id == "SIGN" {
			return nil, fmt.Errorf("cannot safely edit a tag with a protected or flagged %s frame", frame.id)
		}
		if frame.id == "CHAP" || frame.id == "ASPI" || frame.id == "RBUF" {
			offsetFrame = frame.id
		}
		replacement, ok := replacements[frame.id]
		updated := original
		if ok {
			updated = replacement.data
			found[frame.id] = true
		}
		if len(updated) > maxTagSize+tagHeaderSize-len(tag) {
			return nil, errors.New("edited tag exceeds the inspection size limit")
		}
		tag = append(tag, updated...)
	}
	for _, field := range proposal.fields() {
		id := field.id
		replacement, ok := replacements[id]
		if !ok || found[id] {
			continue
		}
		if !replacement.missing {
			return nil, errors.New("approved fields are missing from the original tag")
		}
		if len(replacement.data) > maxTagSize+tagHeaderSize-len(tag) {
			return nil, errors.New("edited tag exceeds the inspection size limit")
		}
		tag = append(tag, replacement.data...)
	}
	// Reuse padding when possible, keeping existing absolute audio offsets valid.
	if originalSize := len(header) + len(body); len(tag) < originalSize {
		tag = append(tag, make([]byte, originalSize-len(tag))...)
	} else if len(tag) > originalSize && offsetFrame != "" {
		return nil, fmt.Errorf("cannot grow a tag with a %s frame that may contain file offsets", offsetFrame)
	}
	putSyncSize(tag[6:tagHeaderSize], len(tag)-tagHeaderSize)
	return tag, nil
}

type approvedFrame struct {
	data    []byte
	missing bool
}

func approvedTextFrames(version byte, proposal songProposal) (map[string]approvedFrame, error) {
	replacements := make(map[string]approvedFrame)
	for _, field := range proposal.fields() {
		if !field.tag.changed() {
			continue
		}
		text, err := encodeTagText(version, field.tag.proposed)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", field.id, err)
		}
		frame := make([]byte, frameHeaderSize, frameHeaderSize+len(text))
		copy(frame, field.id)
		if version == 4 {
			putSyncSize(frame[4:8], len(text))
		} else {
			binary.BigEndian.PutUint32(frame[4:8], uint32(len(text)))
		}
		replacements[field.id] = approvedFrame{data: append(frame, text...), missing: len(field.tag.current) == 0}
	}
	return replacements, nil
}

func encodeTagText(version byte, text string) ([]byte, error) {
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || strings.ContainsFunc(text, unicode.IsControl) {
		return nil, errors.New("expected nonblank Unicode text without control characters")
	}
	if len(text) > maxTagSize-5 {
		return nil, errors.New("text exceeds the inspection size limit")
	}
	if version == 4 {
		return append(append([]byte{3}, text...), 0), nil
	}
	latin1 := true
	for _, r := range text {
		if r > 0xffff {
			return nil, errors.New("ID3v2.3 text cannot represent characters outside the Basic Multilingual Plane")
		}
		latin1 = latin1 && r <= unicode.MaxLatin1
	}
	if latin1 {
		encoded := []byte{0}
		for _, r := range text {
			encoded = append(encoded, byte(r))
		}
		return append(encoded, 0), nil
	}
	encoded := []byte{1, 0xff, 0xfe} // ID3v2.3 Unicode requires a byte order mark.
	for _, r := range text {
		encoded = binary.LittleEndian.AppendUint16(encoded, uint16(r))
	}
	return append(encoded, 0, 0), nil
}
