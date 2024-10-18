package tools

import (
	"os"
	"time"
)

func Mkdir(path string, perm os.FileMode) error {
	_, err := os.Stat(path)
	if err == nil || os.IsExist(err) {
		return nil
	}

	return os.MkdirAll(path, perm)
}

func FileSize(path string) (int64, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}

func FileModTime(path string) (time.Time, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}
