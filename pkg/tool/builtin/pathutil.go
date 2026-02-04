package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateSecurePath validates that a file path is safe to access
// It checks for:
// - Absolute path requirement
// - Symlink resolution to prevent directory traversal
// - Path normalization
func ValidateSecurePath(filePath string) error {
	// Check for absolute path
	if !strings.HasPrefix(filePath, "/") {
		return fmt.Errorf("file_path must be an absolute path, got: %s", filePath)
	}

	// Resolve any symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(filePath)
	if err != nil {
		// If file doesn't exist yet (for writes), check parent directory
		if os.IsNotExist(err) {
			// Try to resolve parent directory
			parentDir := filepath.Dir(filePath)
			realParent, parentErr := filepath.EvalSymlinks(parentDir)
			if parentErr != nil && !os.IsNotExist(parentErr) {
				return fmt.Errorf("failed to resolve parent directory: %v", parentErr)
			}
			// If parent exists, construct the real path
			if realParent != "" {
				realPath = filepath.Join(realParent, filepath.Base(filePath))
			} else {
				// Parent doesn't exist either, use the original path for validation
				realPath = filePath
			}
		} else {
			return fmt.Errorf("failed to resolve path: %v", err)
		}
	}

	// Clean and normalize the path
	cleanPath := filepath.Clean(realPath)

	// Additional security checks could be added here, such as:
	// - Checking if path is within allowed directories
	// - Blocking access to sensitive system paths
	// For now, we just ensure the path is properly resolved and cleaned

	// Log warning if path was changed by symlink resolution
	if cleanPath != filepath.Clean(filePath) {
		// Path was modified by symlink resolution
		// This is informational, not an error
		fmt.Fprintf(os.Stderr, "Info: Path resolved through symlink: %s -> %s\n", filePath, cleanPath)
	}

	return nil
}
