package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const backupSuffix = ".l1ner.bak"

type fixTarget struct {
	path, name   string
	dir          *os.Root
	file         *os.File
	info         os.FileInfo
	digest       [sha256.Size]byte
	header, body []byte
	tags         songTags
}

func openFixTarget(path string, fromAudio bool) (_ *fixTarget, err error) {
	if !strings.EqualFold(filepath.Ext(path), ".mp3") {
		return nil, fmt.Errorf("expected an MP3 file: %s", path)
	}
	dir, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			dir.Close()
		}
	}()
	name := filepath.Base(path)
	file, err := dir.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open fix target: %w", err)
	}
	defer func() {
		if err != nil {
			file.Close()
		}
	}()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("lock fix target: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := validateFixTarget(file, info); err != nil {
		return nil, err
	}
	target := &fixTarget{path: path, name: name, dir: dir, file: file, info: info}
	if err := target.readOriginal(fromAudio); err != nil {
		return nil, err
	}
	return target, nil
}

func validateFixTarget(file *os.File, info os.FileInfo) error {
	if !info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Nlink != 1 {
		return errors.New("fix requires a regular MP3 with one hard link")
	}
	if info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0o200 == 0 {
		return errors.New("fix requires an MP3 you own with owner write permission")
	}
	if info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return errors.New("fix does not support special file mode bits")
	}
	return checkFileAttributes(file)
}

func checkFileAttributes(file *os.File) error {
	// Replacing an inode would lose extended attributes such as ACLs.
	count, err := unix.Flistxattr(int(file.Fd()), nil)
	if err != nil && !errors.Is(err, unix.ENOTSUP) {
		return fmt.Errorf("check file attributes: %w", err)
	}
	if count > 0 {
		return errors.New("fix does not support files with extended attributes")
	}
	return nil
}

func (t *fixTarget) readOriginal(fromAudio bool) error {
	if _, err := t.dir.Lstat(t.name + backupSuffix); err == nil {
		return fmt.Errorf("backup already exists: %s", t.path+backupSuffix)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	header, body, err := readTag(t.file, t.info.Size())
	if err != nil {
		return err
	}
	if header == nil && !fromAudio {
		return errors.New("fix requires an existing ID3v2 tag with title and artist")
	}
	entries, err := parseFrames(header, body)
	if err != nil {
		return err
	}
	if fromAudio {
		t.tags = tagsForAudioReview(entries)
	} else {
		t.tags, err = lookupTagsFromEntries(entries)
		if err != nil {
			return err
		}
	}
	t.digest, err = hashFileSection(t.file, 0, t.info.Size())
	if err != nil {
		return fmt.Errorf("read original MP3: %w", err)
	}
	t.header, t.body = header, body
	return t.checkOriginal()
}

func (t *fixTarget) close() {
	t.file.Close()
	t.dir.Close()
}

func (t *fixTarget) checkOriginal() error {
	current, err := t.dir.Lstat(t.name)
	if err != nil {
		return fmt.Errorf("check reviewed file: %w", err)
	}
	// Also check the user-supplied path in case its parent directory moved.
	byPath, err := os.Lstat(t.path)
	if err != nil {
		return fmt.Errorf("check reviewed path: %w", err)
	}
	if !os.SameFile(t.info, current) || !os.SameFile(current, byPath) || t.info.Size() != byPath.Size() ||
		t.info.Sys().(*syscall.Stat_t).Ctim != byPath.Sys().(*syscall.Stat_t).Ctim {
		return errors.New("MP3 changed since review began")
	}
	return nil
}

type fixResult struct {
	backupPath  string
	syncWarning error
}

func (t *fixTarget) apply(ctx context.Context, proposal songProposal) (fixResult, error) {
	if ctx.Err() != nil {
		return fixResult{}, errPreviewCanceled
	}
	if proposal.path != t.path || !proposal.changed() ||
		!slices.Equal(proposal.title.current, t.tags.titles) || !slices.Equal(proposal.artist.current, t.tags.artists) {
		return fixResult{}, errors.New("proposal does not match the reviewed MP3")
	}
	tag, err := rewriteTag(t.header, t.body, proposal)
	if err != nil {
		return fixResult{}, err
	}
	if err := t.checkOriginal(); err != nil {
		return fixResult{}, err
	}
	backup, backupStage, err := t.stageFile()
	if err != nil {
		return fixResult{}, err
	}
	defer t.dir.Remove(backupStage)
	defer backup.Close()
	if err := t.copyBackup(backup); err != nil {
		return fixResult{}, err
	}
	replacement, replacementStage, err := t.stageFile()
	if err != nil {
		return fixResult{}, err
	}
	defer t.dir.Remove(replacementStage)
	defer replacement.Close()
	if err := t.writeReplacement(replacement, backup, tag, proposal); err != nil {
		return fixResult{}, err
	}
	for _, file := range []*os.File{backup, replacement} {
		if err := finishStage(file, t.info); err != nil {
			return fixResult{}, fmt.Errorf("finish staged MP3: %w", err)
		}
	}
	return t.publish(ctx, backupStage, replacementStage)
}

func (t *fixTarget) stageFile() (*os.File, string, error) {
	name := ".l1ner-stage-" + rand.Text()
	file, err := t.dir.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("stage file beside MP3: %w", err)
	}
	return file, name, nil
}

func (t *fixTarget) copyBackup(backup *os.File) error {
	if _, err := io.CopyN(backup, io.NewSectionReader(t.file, 0, t.info.Size()), t.info.Size()); err != nil {
		return fmt.Errorf("copy original MP3: %w", err)
	}
	digest, err := hashFileSection(backup, 0, t.info.Size())
	if err != nil {
		return fmt.Errorf("verify backup: %w", err)
	}
	if digest != t.digest {
		return errors.New("MP3 changed since review began or its backup could not be verified")
	}
	return nil
}

func (t *fixTarget) writeReplacement(file, backup *os.File, tag []byte, proposal songProposal) error {
	if _, err := file.Write(tag); err != nil {
		return fmt.Errorf("write edited tag: %w", err)
	}
	originalOffset := int64(len(t.header) + len(t.body))
	tailSize := t.info.Size() - originalOffset
	if _, err := io.CopyN(file, io.NewSectionReader(backup, originalOffset, tailSize), tailSize); err != nil {
		return fmt.Errorf("copy audio and trailing data: %w", err)
	}
	return verifyReplacement(file, backup, tag, proposal, originalOffset, tailSize)
}

func verifyReplacement(file, backup *os.File, tag []byte, proposal songProposal, originalOffset, tailSize int64) error {
	newOffset := int64(len(tag))
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != newOffset+tailSize {
		return errors.New("replacement MP3 has an unexpected size")
	}
	writtenTag, err := hashFileSection(file, 0, newOffset)
	if err != nil || writtenTag != sha256.Sum256(tag) {
		return errors.New("replacement tag failed byte verification")
	}
	entries, err := parseFrames(tag[:tagHeaderSize], tag[tagHeaderSize:])
	if err != nil {
		return fmt.Errorf("verify replacement tags: %w", err)
	}
	tags, err := lookupTagsFromEntries(entries)
	if err != nil {
		return err
	}
	if !slices.Equal(tags.titles, expectedTagValues(proposal.title)) || !slices.Equal(tags.artists, expectedTagValues(proposal.artist)) {
		return errors.New("replacement tags differ from the approved proposal")
	}
	before, err := hashFileSection(backup, originalOffset, tailSize)
	if err != nil {
		return fmt.Errorf("verify original audio and trailing data: %w", err)
	}
	after, err := hashFileSection(file, newOffset, tailSize)
	if err != nil {
		return fmt.Errorf("verify replacement audio and trailing data: %w", err)
	}
	if before != after {
		return errors.New("replacement audio or trailing data differs from the original")
	}
	return nil
}

func expectedTagValues(tag tagComparison) []string {
	if tag.changed() {
		return []string{tag.proposed}
	}
	return tag.current
}

func hashFileSection(file *os.File, offset, size int64) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	hash := sha256.New()
	n, err := io.Copy(hash, io.NewSectionReader(file, offset, size))
	if err != nil {
		return digest, err
	}
	if n != size {
		return digest, io.ErrUnexpectedEOF
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func finishStage(file *os.File, original os.FileInfo) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	gid := original.Sys().(*syscall.Stat_t).Gid
	if info.Sys().(*syscall.Stat_t).Gid != gid {
		if err := file.Chown(-1, int(gid)); err != nil {
			return err
		}
	}
	if err := file.Chmod(original.Mode().Perm()); err != nil {
		return err
	}
	if err := checkFileAttributes(file); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}

func (t *fixTarget) publish(ctx context.Context, backupStage, replacementStage string) (fixResult, error) {
	directory, err := t.dir.Open(".")
	if err != nil {
		return fixResult{}, err
	}
	defer directory.Close()
	if err := t.checkOriginal(); err != nil {
		return fixResult{}, err
	}
	if ctx.Err() != nil {
		return fixResult{}, errPreviewCanceled
	}
	backupPath := t.path + backupSuffix
	if err := t.dir.Link(backupStage, t.name+backupSuffix); err != nil {
		return fixResult{}, fmt.Errorf("publish backup: %w", err)
	}
	// Make the exact backup durable before replacing the original directory entry.
	if err := directory.Sync(); err != nil {
		return fixResult{}, fmt.Errorf("sync backup directory (%s): %w", backupPath, err)
	}
	if err := t.verifyOriginal(); err != nil {
		return fixResult{}, fmt.Errorf("%w; backup saved at %s", err, backupPath)
	}
	if ctx.Err() != nil {
		return fixResult{}, fmt.Errorf("fix canceled; backup saved at %s: %w", backupPath, errPreviewCanceled)
	}
	if err := t.dir.Rename(replacementStage, t.name); err != nil {
		return fixResult{}, fmt.Errorf("replace MP3 (backup at %s): %w", backupPath, err)
	}
	return fixResult{backupPath: backupPath, syncWarning: directory.Sync()}, nil
}

func (t *fixTarget) verifyOriginal() error {
	digest, err := hashFileSection(t.file, 0, t.info.Size())
	if err != nil {
		return fmt.Errorf("verify reviewed MP3: %w", err)
	}
	if digest != t.digest {
		return errors.New("MP3 changed since review began")
	}
	return t.checkOriginal()
}
