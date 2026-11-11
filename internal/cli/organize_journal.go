package cli

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const albumJournalVersion = 1

type albumFileID struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func albumIdentity(info os.FileInfo) albumFileID {
	stat := info.Sys().(*syscall.Stat_t)
	return albumFileID{Device: uint64(stat.Dev), Inode: uint64(stat.Ino)}
}

type albumJournalHeader struct {
	Version int         `json:"version"`
	Root    albumFileID `json:"root"`
}

type albumJournalMove struct {
	Source      string            `json:"source"`
	Destination string            `json:"destination"`
	File        albumFileID       `json:"file"`
	SHA256      [sha256.Size]byte `json:"sha256"`
}

type albumJournalFolder struct {
	Path string      `json:"path"`
	File albumFileID `json:"file"`
}

type albumJournalEntry struct {
	Move     *albumJournalMove   `json:"move,omitempty"`
	Folder   *albumJournalFolder `json:"folder,omitempty"`
	Resolved int                 `json:"resolved,omitempty"`
}

type albumJournal struct {
	file    *os.File
	path    string
	root    *os.Root
	entries int
}

// If the recovery record cannot be made durable, stop the batch.
type albumRecoveryError struct{ error }

func (err albumRecoveryError) Unwrap() error { return err.error }

func (w *albumWorkspace) createJournal() (*albumJournal, error) {
	if err := w.verifyRoot(); err != nil {
		return nil, err
	}
	info, err := w.directory.Stat()
	if err != nil {
		return nil, err
	}
	name := ".l1ner-organize-" + rand.Text() + ".jsonl"
	file, err := w.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create organization journal: %w", err)
	}
	journal := &albumJournal{file: file, path: filepath.Join(w.root.Name(), name), root: w.root}
	if err := journal.append(albumJournalHeader{Version: albumJournalVersion, Root: albumIdentity(info)}); err != nil {
		file.Close()
		return nil, err
	}
	if err := w.directory.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("sync journal directory (%s): %w", journal.path, err)
	}
	return journal, nil
}

func (j *albumJournal) append(record any) error {
	if err := json.NewEncoder(j.file).Encode(record); err != nil {
		return albumRecoveryError{fmt.Errorf("write organization journal %s: %w", j.path, err)}
	}
	if err := j.file.Sync(); err != nil {
		return albumRecoveryError{fmt.Errorf("sync organization journal %s: %w", j.path, err)}
	}
	opened, err := j.file.Stat()
	if err != nil {
		return albumRecoveryError{err}
	}
	current, err := j.root.Lstat(filepath.Base(j.path))
	if err != nil {
		return albumRecoveryError{fmt.Errorf("check organization journal %s: %w", j.path, err)}
	}
	if !os.SameFile(opened, current) {
		return albumRecoveryError{fmt.Errorf("organization journal changed: %s", j.path)}
	}
	return nil
}

func (j *albumJournal) record(entry albumJournalEntry) (int, error) {
	if err := j.append(entry); err != nil {
		return 0, err
	}
	j.entries++
	return j.entries, nil
}

func (w *albumWorkspace) openJournal(name string) (_ *albumJournal, _ []albumJournalEntry, err error) {
	file, err := w.root.OpenFile(name, os.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open organization journal: %w", err)
	}
	defer func() {
		if err != nil {
			file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0o022 != 0 {
		return nil, nil, errors.New("undo requires an organization journal you own with owner-only write access")
	}
	rootInfo, err := w.directory.Stat()
	if err != nil {
		return nil, nil, err
	}
	entries, end, err := readAlbumJournal(file, albumIdentity(rootInfo))
	if err != nil {
		return nil, nil, err
	}
	// Append after the last complete line if an interrupted write left a partial record.
	if end < info.Size() {
		if err := file.Truncate(end); err != nil {
			return nil, nil, fmt.Errorf("discard incomplete journal line: %w", err)
		}
	}
	if _, err := file.Seek(end, io.SeekStart); err != nil {
		return nil, nil, err
	}
	return &albumJournal{file: file, path: filepath.Join(w.root.Name(), name), root: w.root, entries: len(entries)}, entries, nil
}

func readAlbumJournal(file *os.File, root albumFileID) ([]albumJournalEntry, int64, error) {
	reader := bufio.NewReaderSize(file, 16<<10)
	var header albumJournalHeader
	line, err := reader.ReadSlice('\n')
	if err != nil || json.Unmarshal(line, &header) != nil || header.Version != albumJournalVersion {
		return nil, 0, errors.New("invalid or unsupported organization journal header")
	}
	if header.Root != root {
		return nil, 0, errors.New("journal belongs to a different organization folder")
	}
	end := int64(len(line))
	var entries []albumJournalEntry
	for lineNumber := 2; ; lineNumber++ {
		line, err := reader.ReadSlice('\n')
		if errors.Is(err, io.EOF) {
			// A crash can leave a partial final record. A rename requires a synced complete record.
			return entries, end, nil
		}
		if err != nil {
			return nil, 0, fmt.Errorf("read journal line %d: %w", lineNumber, err)
		}
		var entry albumJournalEntry
		if json.Unmarshal(line, &entry) != nil || !validAlbumJournalEntry(entry, entries) {
			return nil, 0, fmt.Errorf("invalid organization journal entry on line %d", lineNumber)
		}
		entries = append(entries, entry)
		end += int64(len(line))
	}
}

func validAlbumJournalEntry(entry albumJournalEntry, previous []albumJournalEntry) bool {
	if entry.Resolved != 0 {
		return entry.Move == nil && entry.Folder == nil && entry.Resolved > 0 &&
			entry.Resolved <= len(previous) && previous[entry.Resolved-1].Resolved == 0
	}
	if entry.Move != nil && entry.Folder == nil {
		move := entry.Move
		parts := strings.Split(move.Destination, "/")
		return move.File.Inode != 0 && len(parts) == 3 && parts[2] == move.Source &&
			strings.EqualFold(filepath.Ext(move.Source), ".mp3") &&
			validAlbumComponent(parts[0]) && validAlbumComponent(parts[1]) && validAlbumComponent(parts[2])
	}
	if entry.Folder != nil && entry.Move == nil {
		parts := strings.Split(entry.Folder.Path, "/")
		return entry.Folder.File.Inode != 0 && len(parts) <= 2 &&
			validAlbumComponent(parts[0]) && (len(parts) == 1 || validAlbumComponent(parts[1]))
	}
	return false
}
