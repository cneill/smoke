package fs

import (
	"errors"
	"path/filepath"
	"strings"
)

var (
	ErrNonAbsoluteProjectPath = errors.New("project path is not an absolute path")
	ErrInsecureProjectPath    = errors.New("project path is insecure")
	ErrInsecureTargetPath     = errors.New("target path is insecure")
	ErrOutsideBounds          = errors.New("final path is not inside project directory")
)

// GetRelativePath ensures that 'targetPath' does not attempt to escape 'projectPath', and returns the combined path if
// it can be done safely.
func GetRelativePath(projectPath, targetPath string) (string, error) {
	if projectPath == "" || projectPath == "/" {
		return "", ErrInsecureProjectPath
	}

	if hasTraversal(targetPath) {
		return "", ErrInsecureTargetPath
	}

	if !filepath.IsAbs(projectPath) {
		return "", ErrNonAbsoluteProjectPath
	}

	cleaned := filepath.Clean(targetPath)
	fullPath := filepath.Join(projectPath, cleaned)

	rel, err := filepath.Rel(projectPath, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrOutsideBounds
	}

	return fullPath, nil
}

func hasTraversal(targetPath string) bool {
	return strings.HasPrefix(targetPath, "..") ||
		strings.Contains(targetPath, "../") ||
		strings.Contains(targetPath, "..\\") ||
		strings.Contains(targetPath, "/..") ||
		strings.Contains(targetPath, "\\..")
}
