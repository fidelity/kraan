package utils

import (
	"fmt"
	"os"
)

func IsExistingDir(dataPath string) error {
	info, err := os.Stat(dataPath)
	if os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("addons Data path: %s, is not a directory", dataPath)
	}

	return nil
}
