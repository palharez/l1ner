package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type scanResult struct {
	paths  []string
	errors []error
}

func scanDirectory(root string, recursive bool) scanResult {
	var result scanResult
	info, err := os.Lstat(filepath.Clean(root))
	if err != nil {
		result.errors = append(result.errors, err)
		return result
	}
	// WalkDir does not follow directory symlinks, including a symlink at the root.
	if info.Mode()&os.ModeSymlink != 0 {
		result.errors = append(result.errors, fmt.Errorf("directory path is a symbolic link: %s", root))
		return result
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			result.errors = append(result.errors, fmt.Errorf("scan %s: %w", path, err))
			return nil
		}
		if entry.IsDir() {
			if path != root && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".mp3") {
			result.paths = append(result.paths, path)
		}
		return nil
	})
	if err != nil {
		result.errors = append(result.errors, err)
	}
	return result
}

func inspectDirectory(options inspectOptions, out, errOut io.Writer) int {
	scan := scanDirectory(options.path, options.recursive)
	for _, err := range scan.errors {
		reportError(errOut, err)
	}
	inspected, failed, err := inspectBatch(scan.paths, out, errOut)
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	if len(scan.paths) == 0 && len(scan.errors) == 0 {
		if _, err := fmt.Fprintln(out, "No MP3 files found."); err != nil {
			reportError(errOut, err)
			return 1
		}
	}
	if _, err := fmt.Fprintf(out, "Summary: %d inspected, %d failed, %d scan errors\n", inspected, failed, len(scan.errors)); err != nil {
		reportError(errOut, err)
		return 1
	}
	if failed > 0 || len(scan.errors) > 0 {
		return 1
	}
	return 0
}

func inspectBatch(paths []string, out, errOut io.Writer) (inspected, failed int, err error) {
	for _, path := range paths {
		entries, err := inspect(path)
		if err != nil {
			failed++
			reportError(errOut, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if inspected > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return inspected, failed, err
			}
		}
		if err := display(out, path, entries); err != nil {
			return inspected, failed, err
		}
		inspected++
	}
	return inspected, failed, nil
}
