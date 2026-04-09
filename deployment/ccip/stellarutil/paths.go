package stellarutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindStellarRoot locates the chainlink-stellar project root by walking up from
// CWD looking for go.mod. This works whether the devenv CLI is run from the
// chainlink-stellar root directly or from a subdirectory.
func FindStellarRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "target")); err == nil {
				return dir, nil
			}
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod in any parent of %s", dir)
		}
		dir = parent
	}
}
