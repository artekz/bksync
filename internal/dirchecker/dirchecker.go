package dirchecker

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type HashEntry struct {
	Directory string `json:"directory"`
	Hash      string `json:"hash"`
}

type DirChecker struct {
	hashFilePath string
	hashes       map[string]string
	loaded       bool
}

func NewDirChecker(hashFilePath string) *DirChecker {
	return &DirChecker{hashFilePath: hashFilePath}
}

func (c *DirChecker) HashChanged(path string) (string, bool, error) {
	if err := c.loadHashes(); err != nil {
		return "", false, err
	}

	hash, err := c.GenerateDirHash(path)
	if err != nil {
		return "", false, err
	}

	existingHash, exists := c.hashes[path]
	return hash, !exists || existingHash != hash, nil
}

func (c *DirChecker) StoreHash(path, hash string) error {
	if err := c.loadHashes(); err != nil {
		return err
	}

	hashes := make(map[string]string, len(c.hashes)+1)
	for directory, existingHash := range c.hashes {
		hashes[directory] = existingHash
	}
	hashes[path] = hash

	entries := make([]HashEntry, 0, len(hashes))
	for directory, entryHash := range hashes {
		entries = append(entries, HashEntry{Directory: directory, Hash: entryHash})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Directory < entries[j].Directory
	})

	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := c.writeHashes(data); err != nil {
		return err
	}

	c.hashes = hashes
	return nil
}

func (c *DirChecker) GenerateDirHash(root string) (string, error) {
	root = filepath.Clean(root)
	hasher := sha256.New()
	err := filepath.WalkDir(root, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if filePath != root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}

		if info.IsDir() {
			_, err = fmt.Fprintf(hasher, "dir:%q\n", relativePath)
			return err
		}

		_, err = fmt.Fprintf(hasher, "file:%q:%d:%d\n", relativePath, info.Size(), info.ModTime().UnixNano())
		return err
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func (c *DirChecker) loadHashes() error {
	if c.loaded {
		return nil
	}
	if c.hashFilePath == "" {
		return fmt.Errorf("hash file path is empty")
	}

	data, err := os.ReadFile(c.hashFilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var entries []HashEntry
	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
	}

	c.hashes = make(map[string]string, len(entries))
	for _, entry := range entries {
		c.hashes[entry.Directory] = entry.Hash
	}
	c.loaded = true
	return nil
}

func (c *DirChecker) writeHashes(data []byte) error {
	dir := filepath.Dir(c.hashFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.CreateTemp(dir, ".hashes-*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(tempPath, c.hashFilePath)
}
