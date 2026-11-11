package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sys/unix"
	"golang.org/x/text/cases"
)

type albumPlan struct {
	root       string
	moves      []albumMove
	scanErrors []error
}

type albumMove struct {
	source, destination string
	blocked             string
	err                 error
}

var foldAlbumPath = cases.Fold()

func planAlbumFolders(path string) (albumPlan, error) {
	dir, err := openAlbumRoot(path)
	if err != nil {
		return albumPlan{}, err
	}
	defer dir.Close()
	return planAlbumRoot(dir), nil
}

func openAlbumRoot(path string) (*os.Root, error) {
	root := filepath.Clean(path)
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("directory path is a symbolic link: %s", root)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("expected a directory: %s", root)
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	opened, err := dir.Stat(".")
	if err != nil {
		dir.Close()
		return nil, err
	}
	if !os.SameFile(info, opened) {
		dir.Close()
		return nil, fmt.Errorf("selected directory changed while opening: %s", root)
	}
	return dir, nil
}

func planAlbumRoot(dir *os.Root) albumPlan {
	root := dir.Name()
	scan := scanDirectory(root, false)
	plan := albumPlan{root: root, scanErrors: scan.errors}
	for _, path := range scan.paths {
		plan.moves = append(plan.moves, planAlbumMove(dir, filepath.Base(path)))
	}
	checkAlbumDestinations(dir.FS(), plan.moves)
	return plan
}

func planAlbumMove(dir *os.Root, source string) albumMove {
	file, err := dir.OpenFile(source, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return albumMove{source: source, err: err}
	}
	defer file.Close()
	return albumMoveFromFile(file, source)
}

func albumMoveFromFile(file *os.File, source string) albumMove {
	move := albumMove{source: source}
	entries, err := inspectOpenMP3(file)
	if err != nil {
		move.err = err
		return move
	}
	parts := make([]string, 0, 3)
	for _, field := range []struct{ id, label string }{{"TPE2", "album artist"}, {"TALB", "album"}} {
		values, err := lookupTag(entries, field.id, field.label)
		if err != nil {
			move.blocked = err.Error()
			return move
		}
		name := strings.TrimSpace(values[0])
		if !validAlbumComponent(name) {
			move.blocked = fmt.Sprintf("unsafe %s folder name %q", field.label, name)
			return move
		}
		parts = append(parts, name)
	}
	if !validAlbumComponent(source) {
		move.blocked = "unsafe source filename"
		return move
	}
	move.destination = filepath.Join(append(parts, source)...)
	return move
}

func validAlbumComponent(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 &&
		utf8.ValidString(name) && !strings.ContainsAny(name, "/\\") &&
		!strings.ContainsFunc(name, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) })
}

func checkAlbumDestinations(root fs.FS, moves []albumMove) {
	conflicts := albumPathConflicts(moves)
	names := albumDirectoryNames{root: root}
	for i := range moves {
		move := &moves[i]
		if move.destination == "" {
			continue
		}
		for path := move.destination; path != "."; path = filepath.Dir(path) {
			if conflicts[foldAlbumPath.String(path)] {
				move.blocked = fmt.Sprintf("planned paths collide at %q", path)
				break
			}
		}
		if move.blocked == "" {
			move.blocked, move.err = names.check(move.destination)
		}
	}
}

func albumPathConflicts(moves []albumMove) map[string]bool {
	claimed := make(map[string]string)
	conflicts := make(map[string]bool)
	for _, move := range moves {
		if move.destination == "" {
			continue
		}
		for path := move.destination; path != "."; path = filepath.Dir(path) {
			key := foldAlbumPath.String(path)
			if previous, ok := claimed[key]; ok && (previous != path || path == move.destination) {
				conflicts[key] = true
			}
			claimed[key] = path
		}
	}
	return conflicts
}

type albumDirectoryNames struct {
	root        fs.FS
	directories map[string]map[string][]fs.DirEntry
}

func (names *albumDirectoryNames) check(destination string) (string, error) {
	parts := strings.Split(destination, string(filepath.Separator))
	parent := "."
	for i, name := range parts {
		entries, err := names.read(parent)
		if err != nil {
			return "", fmt.Errorf("read destination folder %q: %w", parent, err)
		}
		matches := entries[foldAlbumPath.String(name)]
		if len(matches) == 0 {
			return "", nil
		}
		path := filepath.Join(parent, name)
		if len(matches) > 1 || matches[0].Name() != name {
			return fmt.Sprintf("destination has a case-insensitive collision at %q", path), nil
		}
		if matches[0].Type()&os.ModeSymlink != 0 {
			return fmt.Sprintf("destination path includes a symbolic link at %q", path), nil
		}
		if i == len(parts)-1 {
			return fmt.Sprintf("destination already exists at %q", path), nil
		}
		if !matches[0].IsDir() {
			return fmt.Sprintf("destination parent is not a directory at %q", path), nil
		}
		parent = path
	}
	return "", nil
}

func (names *albumDirectoryNames) read(parent string) (map[string][]fs.DirEntry, error) {
	if entries, ok := names.directories[parent]; ok {
		return entries, nil
	}
	children, err := fs.ReadDir(names.root, parent)
	if err != nil {
		return nil, err
	}
	entries := make(map[string][]fs.DirEntry)
	for _, child := range children {
		key := foldAlbumPath.String(child.Name())
		entries[key] = append(entries[key], child)
	}
	if names.directories == nil {
		names.directories = make(map[string]map[string][]fs.DirEntry)
	}
	names.directories[parent] = entries
	return entries, nil
}
