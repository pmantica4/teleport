package utils

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetGitDirectory returns the path of the current git directory.
func GetGitDirectory() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	// Remove any trailing whitespace (like newlines)
	return strings.TrimSpace(out.String()), nil
}

func GetRelativePath(path string) (string) {
	dir, err := GetGitDirectory()
	if err != nil {
		panic(err)
	}
	return filepath.Join(dir, path)
}
