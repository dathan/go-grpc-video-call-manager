package utils

import (
	"os"
)

func GetFileContents(paths []string) ([]byte, error) {
	for _, path := range paths {
		// Attempt to read the file at the path
		b, err := os.ReadFile(path)
		if err == nil {
			return b, nil // Return the contents and nil error if the file is found
		}
	}
	// Return nil and an error if the file is not found in any of the paths
	return nil, os.ErrNotExist
}
