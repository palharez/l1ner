package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type albumWorkspace struct {
	root      *os.Root
	directory *os.File
}

func openAlbumWorkspace(path string) (*albumWorkspace, error) {
	root, err := openAlbumRoot(path)
	if err != nil {
		return nil, err
	}
	directory, err := root.Open(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	w := &albumWorkspace{root: root, directory: directory}
	if err := unix.Flock(int(directory.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		w.close()
		return nil, fmt.Errorf("lock organization folder: %w", err)
	}
	return w, nil
}

func (w *albumWorkspace) close() {
	w.directory.Close()
	w.root.Close()
}

func (w *albumWorkspace) verifyRoot() error {
	info, err := w.directory.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(w.root.Name())
	if err != nil {
		return err
	}
	if !os.SameFile(info, current) {
		return errors.New("selected directory changed during organization")
	}
	return nil
}

type reviewedAlbumSong struct {
	move albumMove
	file *os.File
	info os.FileInfo
}

type albumMoveBlocked struct{ error }

func (err albumMoveBlocked) Unwrap() error { return err.error }

func (w *albumWorkspace) reviewSong(move albumMove) (_ *reviewedAlbumSong, err error) {
	file, err := openAlbumFileAt(w.directory, move.source)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			file.Close()
		}
	}()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("lock MP3 for review: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	current := albumMoveFromFile(file, move.source)
	if current.err != nil {
		return nil, current.err
	}
	if current.destination != move.destination || current.blocked != "" {
		return nil, albumMoveBlocked{errors.New("album tags changed since planning")}
	}
	review := &reviewedAlbumSong{move: move, file: file, info: info}
	if err := verifyAlbumFile(w.directory, move.source, info); err != nil {
		return nil, err
	}
	return review, nil
}

func verifyAlbumFile(parent *os.File, name string, reviewed os.FileInfo) error {
	var info unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &info, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return albumMoveBlocked{fmt.Errorf("recheck reviewed MP3: %w", err)}
	}
	before := reviewed.Sys().(*syscall.Stat_t)
	if uint64(info.Dev) != uint64(before.Dev) || info.Ino != before.Ino || info.Size != reviewed.Size() ||
		info.Ctim.Sec != before.Ctim.Sec || info.Ctim.Nsec != before.Ctim.Nsec {
		return albumMoveBlocked{errors.New("MP3 changed during review")}
	}
	return nil
}

func (w *albumWorkspace) moveSong(ctx context.Context, song *reviewedAlbumSong, journal *albumJournal) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := w.verifyRoot(); err != nil {
		return false, albumRecoveryError{err}
	}
	move := song.move
	if err := verifyAlbumFile(w.directory, move.source, song.info); err != nil {
		return false, err
	}
	digest, err := hashFileSection(song.file, 0, song.info.Size())
	if err != nil {
		return false, fmt.Errorf("read approved MP3: %w", err)
	}
	names := albumDirectoryNames{root: w.root.FS()}
	if reason, err := names.check(move.destination); err != nil {
		return false, err
	} else if reason != "" {
		return false, albumMoveBlocked{errors.New(reason)}
	}
	folderPath := filepath.Dir(move.destination)
	folder, err := openAlbumFolder(w.directory, folderPath, journal)
	if err != nil {
		return false, err
	}
	defer folder.Close()
	record := albumJournalMove{Source: move.source, Destination: move.destination, File: albumIdentity(song.info), SHA256: digest}
	if err := journal.append(albumJournalEntry{Move: &record}); err != nil {
		return false, err
	}
	if err := w.verifyRoot(); err != nil {
		return false, albumRecoveryError{err}
	}
	if err := verifyAlbumFolder(w.directory, folderPath, folder); err != nil {
		return false, err
	}
	if err := verifyAlbumFile(w.directory, move.source, song.info); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return renameAlbumFile(w.directory, folder, move.source)
}

func openAlbumFileAt(parent *os.File, name string) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), filepath.Join(parent.Name(), name)), nil
}

func openAlbumDirAt(parent *os.File, name string) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), filepath.Join(parent.Name(), name)), nil
}

// Open each component by directory descriptor so an intermediate symlink cannot redirect a move.
func openAlbumFolder(root *os.File, path string, journal *albumJournal) (*os.File, error) {
	current, err := openAlbumDirAt(root, ".")
	if err != nil || path == "." {
		return current, err
	}
	relative := ""
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		relative = filepath.Join(relative, component)
		next, err := openAlbumDirAt(current, component)
		if errors.Is(err, unix.ENOENT) && journal != nil {
			next, err = createAlbumFolder(current, component, relative, journal)
		}
		current.Close()
		if err != nil {
			return nil, fmt.Errorf("open album folder %q: %w", relative, err)
		}
		current = next
	}
	return current, nil
}

func createAlbumFolder(parent *os.File, name, relative string, journal *albumJournal) (_ *os.File, err error) {
	if err := unix.Mkdirat(int(parent.Fd()), name, 0o755); err != nil {
		return nil, err
	}
	folder, err := openAlbumDirAt(parent, name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			folder.Close()
		}
	}()
	info, err := folder.Stat()
	if err != nil {
		return nil, err
	}
	record := albumJournalFolder{Path: relative, File: albumIdentity(info)}
	if err := journal.append(albumJournalEntry{Folder: &record}); err != nil {
		return nil, err
	}
	if err := parent.Sync(); err != nil {
		return nil, albumRecoveryError{fmt.Errorf("sync new album folder entry: %w", err)}
	}
	return folder, nil
}

func verifyAlbumFolder(root *os.File, path string, folder *os.File) error {
	current, err := openAlbumFolder(root, path, nil)
	if err != nil {
		return albumMoveBlocked{err}
	}
	defer current.Close()
	info, err := current.Stat()
	if err != nil {
		return err
	}
	opened, err := folder.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return albumMoveBlocked{errors.New("album folder changed during review")}
	}
	return nil
}

func renameAlbumFile(from, to *os.File, name string) (bool, error) {
	err := unix.Renameat2(int(from.Fd()), name, int(to.Fd()), name, unix.RENAME_NOREPLACE)
	switch {
	case errors.Is(err, unix.EEXIST):
		return false, albumMoveBlocked{errors.New("destination already exists")}
	case errors.Is(err, unix.EXDEV):
		return false, albumMoveBlocked{errors.New("destination is on a different filesystem")}
	case err != nil:
		return false, fmt.Errorf("move MP3 without replacing the destination: %w", err)
	}
	if err := errors.Join(from.Sync(), to.Sync()); err != nil {
		return true, albumRecoveryError{fmt.Errorf("MP3 moved but directory sync failed: %w", err)}
	}
	return true, nil
}
