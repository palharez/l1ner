package main

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/bogem/id3v2/v2"
)

type frameHeader struct {
	id       string
	bodySize int
}

func parseFrames(tagHeader, body []byte) ([]entry, error) {
	var entries []entry
	for len(body) > 0 {
		if body[0] == 0 {
			if len(bytes.Trim(body, "\x00")) != 0 {
				return nil, fmt.Errorf("malformed ID3 padding")
			}
			break
		}
		header, err := parseFrameHeader(body, tagHeader[3])
		if err != nil {
			return nil, err
		}
		frameSize := frameHeaderSize + header.bodySize
		decoded, err := decodeFrame(tagHeader, header.id, body[:frameSize])
		if err != nil {
			return nil, err
		}
		entries = append(entries, decoded...)
		body = body[frameSize:]
	}
	return entries, nil
}

func parseFrameHeader(body []byte, version byte) (frameHeader, error) {
	if len(body) < frameHeaderSize {
		return frameHeader{}, fmt.Errorf("malformed ID3 frame header: truncated")
	}
	id := string(body[:4])
	for _, char := range id {
		if !(char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return frameHeader{}, fmt.Errorf("malformed ID3 frame identifier")
		}
	}
	bodySize, err := frameBodySize(body[4:8], version)
	if err != nil {
		return frameHeader{}, err
	}
	// A large ID3v2.3 size can become negative when converted to int on 32-bit systems.
	if bodySize <= 0 || bodySize > len(body)-frameHeaderSize {
		return frameHeader{}, fmt.Errorf("malformed %s frame size", id)
	}
	if body[9] != 0 {
		return frameHeader{}, fmt.Errorf("unsupported %s frame format flags: 0x%02x", id, body[9])
	}
	return frameHeader{id: id, bodySize: bodySize}, nil
}

func frameBodySize(data []byte, version byte) (int, error) {
	if version == 4 {
		return syncSize(data)
	}
	return int(binary.BigEndian.Uint32(data)), nil
}

func decodeFrame(tagHeader []byte, id string, data []byte) ([]entry, error) {
	// Preserve nested data without invoking the library's limited chapter parser.
	if id == "CHAP" || id == "CTOC" {
		return []entry{{id: id, frame: id3v2.UnknownFrame{Body: data[frameHeaderSize:]}}}, nil
	}

	// Parse separately so the library cannot collapse duplicate frame identifiers.
	header := append([]byte(nil), tagHeader...)
	putSyncSize(header[6:tagHeaderSize], len(data))
	tagData := append(header, data...)
	tag, err := id3v2.ParseReader(bytes.NewReader(tagData), id3v2.Options{Parse: true})
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", id, err)
	}
	var entries []entry
	for _, frame := range tag.GetFrames(id) {
		entries = append(entries, entry{id: id, frame: frame})
	}
	return entries, nil
}

func syncSize(data []byte) (int, error) {
	size := 0
	for _, value := range data {
		if value&0x80 != 0 {
			return 0, fmt.Errorf("malformed ID3 synchsafe size")
		}
		size = size<<7 | int(value)
	}
	return size, nil
}

func putSyncSize(data []byte, size int) {
	for i := len(data) - 1; i >= 0; i-- {
		data[i] = byte(size & 127)
		size >>= 7
	}
}
