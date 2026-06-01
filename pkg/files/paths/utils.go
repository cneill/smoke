package paths

import (
	"errors"
	"fmt"
	"os"
)

// OptionalRegularFile returns:
//   - true and nil error if file exists and is regular
//   - false and nil error if file does not exist
//   - true and error if file exists but cannot be stat'd or if it's non-regular
func OptionalRegularFile(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return true, fmt.Errorf("error checking for regular file: %w", err)
	}

	if mode := stat.Mode(); !mode.IsRegular() {
		return true, fmt.Errorf("non-regular file: %w", err)
	}

	return true, nil
}
