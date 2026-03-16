// Package fileops provides utility functions for filesystem operations,
// including checking file existence, reading lists of filenames, and directory management.
package fileops

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// FileExistsAndNotEmpty checks if the file exists and is not empty.
func FileExistsAndNotEmpty(filename string) bool {
	info, err := os.Stat(filepath.Clean(filename))
	if os.IsNotExist(err) || info.Size() == 0 {
		return false
	}
	return true
}

// ReadFileNames reads file names from the given file.
// It expects one filename per line and trims surrounding whitespace.
func ReadFileNames(filename string) (names []string, err error) {
	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			names = append(names, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

// CreateDirectory creates a directory if it doesn't exist.
func CreateDirectory(path string) error {
	return os.MkdirAll(path, 0750)
}
