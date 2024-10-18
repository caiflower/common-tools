package tools

import (
	"os"
	"time"
)

func Mkdir(path string, perm os.FileMode) (e error) {
	_, er := os.Stat(path)
	b := er == nil || os.IsExist(er)
	if !b {
		if err := os.MkdirAll(path, perm); err != nil {
			if os.IsPermission(err) {
				e = err
			}
		}
	}
	return
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

func FileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}
