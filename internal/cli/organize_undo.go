package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

type albumUndoSummary struct {
	restored, resolved, blocked, failed int
}

func runOrganizeUndo(path string, out, errOut io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	path = filepath.Clean(path)
	w, err := openAlbumWorkspace(filepath.Dir(path))
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	defer w.close()
	journal, entries, err := w.openJournal(filepath.Base(path))
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	defer journal.file.Close()
	summary, err := w.undo(ctx, journal, entries, out, errOut)
	code := 0
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(errOut, "Undo canceled.")
		code = 130
	case err != nil:
		reportError(errOut, err)
		code = 1
	case summary.blocked > 0 || summary.failed > 0:
		code = 1
	}
	if _, err := fmt.Fprintf(out, "Summary: %d restored, %d already resolved, %d blocked, %d failed\n",
		summary.restored, summary.resolved, summary.blocked, summary.failed); err != nil {
		reportError(errOut, err)
		return 1
	}
	return code
}

func (w *albumWorkspace) undo(ctx context.Context, journal *albumJournal, entries []albumJournalEntry, out, errOut io.Writer) (albumUndoSummary, error) {
	var summary albumUndoSummary
	resolved := make(map[int]bool)
	for _, entry := range entries {
		if entry.Resolved != 0 {
			resolved[entry.Resolved] = true
		}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		entry := entries[i]
		if entry.Resolved != 0 {
			continue
		}
		if resolved[i+1] {
			if entry.Move != nil {
				summary.resolved++
			}
			continue
		}
		if entry.Folder != nil {
			done, err := w.removeRecordedFolder(*entry.Folder)
			if err != nil {
				summary.failed++
				if err := reportError(errOut, fmt.Errorf("remove recorded folder %q: %w", entry.Folder.Path, err)); err != nil {
					return summary, err
				}
			} else if done {
				if _, err := journal.record(albumJournalEntry{Resolved: i + 1}); err != nil {
					return summary, err
				}
			}
			continue
		}
		move := *entry.Move
		restored, err := w.restoreSong(ctx, move)
		if restored {
			summary.restored++
		}
		if err == nil {
			if _, err := journal.record(albumJournalEntry{Resolved: i + 1}); err != nil {
				return summary, err
			}
			if restored {
				if _, err := fmt.Fprintf(out, "Restored: %s\n", safe(filepath.Join(w.root.Name(), move.Source))); err != nil {
					return summary, err
				}
			} else {
				summary.resolved++
			}
			continue
		}
		var recoveryErr albumRecoveryError
		if errors.Is(err, context.Canceled) || errors.As(err, &recoveryErr) {
			return summary, err
		}
		var blocked albumMoveBlocked
		if errors.As(err, &blocked) {
			summary.blocked++
		} else {
			summary.failed++
		}
		if err := reportError(errOut, fmt.Errorf("restore %q: %w", move.Source, err)); err != nil {
			return summary, err
		}
	}
	return summary, ctx.Err()
}

func (w *albumWorkspace) restoreSong(ctx context.Context, move albumJournalMove) (bool, error) {
	if err := w.verifyRoot(); err != nil {
		return false, albumRecoveryError{err}
	}
	original, err := openAlbumFileAt(w.directory, move.Source)
	if err == nil {
		defer original.Close()
		if _, err := verifyRecordedSong(original, move); err != nil {
			return false, albumMoveBlocked{fmt.Errorf("original path is occupied: %w", err)}
		}
		return false, nil
	}
	if !errors.Is(err, unix.ENOENT) {
		return false, albumMoveBlocked{fmt.Errorf("check original path: %w", err)}
	}
	names := albumDirectoryNames{root: w.root.FS()}
	if reason, err := names.check(move.Source); err != nil {
		return false, err
	} else if reason != "" {
		return false, albumMoveBlocked{errors.New(reason)}
	}
	folderPath := filepath.Dir(move.Destination)
	folder, err := openAlbumFolder(w.directory, folderPath, nil)
	if err != nil {
		return false, albumMoveBlocked{fmt.Errorf("locate moved MP3: %w", err)}
	}
	defer folder.Close()
	file, err := openAlbumFileAt(folder, move.Source)
	if err != nil {
		return false, albumMoveBlocked{fmt.Errorf("open moved MP3: %w", err)}
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return false, fmt.Errorf("lock moved MP3: %w", err)
	}
	info, err := verifyRecordedSong(file, move)
	if err != nil {
		return false, albumMoveBlocked{err}
	}
	if err := w.verifyRoot(); err != nil {
		return false, albumRecoveryError{err}
	}
	if err := verifyAlbumFolder(w.directory, folderPath, folder); err != nil {
		return false, err
	}
	if err := verifyAlbumFile(folder, move.Source, info); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return renameAlbumFile(folder, w.directory, move.Source)
}

func verifyRecordedSong(file *os.File, move albumJournalMove) (os.FileInfo, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || albumIdentity(info) != move.File {
		return nil, errors.New("file does not match the recorded MP3")
	}
	digest, err := hashFileSection(file, 0, info.Size())
	if err != nil {
		return nil, err
	}
	if digest != move.SHA256 {
		return nil, errors.New("MP3 contents differ from the journal")
	}
	return info, nil
}

func (w *albumWorkspace) removeRecordedFolder(record albumJournalFolder) (bool, error) {
	if err := w.verifyRoot(); err != nil {
		return false, err
	}
	parent, err := openAlbumFolder(w.directory, filepath.Dir(record.Path), nil)
	if errors.Is(err, unix.ENOENT) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer parent.Close()
	name := filepath.Base(record.Path)
	var info unix.Stat_t
	err = unix.Fstatat(int(parent.Fd()), name, &info, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFDIR || (albumFileID{Device: uint64(info.Dev), Inode: info.Ino}) != record.File {
		return true, nil
	}
	err = unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
	if errors.Is(err, unix.ENOENT) {
		return true, nil
	}
	if errors.Is(err, unix.ENOTEMPTY) || errors.Is(err, unix.EEXIST) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, parent.Sync()
}
