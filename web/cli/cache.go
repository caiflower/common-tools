/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const defaultCacheTTL = 5 * time.Minute

type cacheFile struct {
	Metadata
	FetchedAt time.Time `json:"fetchedAt"`
}

func cacheFilePath(server, dir string) (string, error) {
	if dir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cacheDir, "web-cli")
	}
	sum := sha256.Sum256([]byte(server))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), nil
}

func readCacheFile(path string) (*cacheFile, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cached cacheFile
	if err := json.Unmarshal(body, &cached); err != nil {
		return nil, err
	}
	return &cached, nil
}

func writeCacheFile(path string, metadata Metadata, fetchedAt time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.Marshal(cacheFile{Metadata: metadata, FetchedAt: fetchedAt})
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}
