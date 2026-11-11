package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
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
	Move   *albumJournalMove   `json:"move,omitempty"`
	Folder *albumJournalFolder `json:"folder,omitempty"`
}

type albumJournal struct {
	file *os.File
	path string
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
	journal := &albumJournal{file: file, path: filepath.Join(w.root.Name(), name)}
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
	return nil
}
