package citest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// WriteSummaryFile atomically publishes only the structured Summary. Existing
// links and non-regular destinations are rejected rather than followed.
func WriteSummaryFile(path string, summary Summary) error {
	if path == "" {
		return errors.New("summary path is empty")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	directoryInfo, err := os.Lstat(directory)
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("summary directory is not a real directory")
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("summary destination is not a regular file")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}

	temporary, err := os.CreateTemp(directory, ".intent-sh-summary-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(summary); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}
