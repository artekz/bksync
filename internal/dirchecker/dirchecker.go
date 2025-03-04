package dirchecker

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const hashFileName = "../hashes.json"

type HashEntry struct {
	Directory string `json:"directory"`
	Hash      string `json:"hash"`
}

type DirChecker struct {
	existingHashes []*HashEntry
}

func NewDirChecker() *DirChecker {
	return &DirChecker{}
}

func (c *DirChecker) Check(path string) (bool, error) {
	existingHashes, err := c.getExistingHashes()
	if err != nil {
		return false, err
	}

	hash, err := c.GenerateDirHash(path)
	if err != nil {
		return false, err
	}

	// Check if the hash has changed
	hashChanged := true
	for _, entry := range existingHashes {
		if entry.Directory == path && entry.Hash == hash {
			hashChanged = false
			break
		}
	}

	return hashChanged, nil
}

func (c *DirChecker) StoreHash(path, hash string) error {
	existingHashes, err := c.getExistingHashes()
	if err != nil {
		return err
	}

	for i, existingHash := range existingHashes {
		if existingHash.Directory == path {
			existingHashes = append(existingHashes[:i], existingHashes[i+1:]...)
		}
	}

	// Store the new hash in the hashFile
	hashEntry := HashEntry{
		Directory: path,
		Hash:      hash,
	}
	existingHashes = append(existingHashes, &hashEntry)
	c.existingHashes = existingHashes
	existingHashesJSON, err := json.Marshal(c.existingHashes)
	if err != nil {
		return err
	}

	hashFile, err := os.OpenFile(hashFileName, os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer hashFile.Close()

	_, err = hashFile.Write(existingHashesJSON)
	if err != nil {
		return err
	}
	_, err = hashFile.WriteString("\n")
	if err != nil {
		return err
	}

	return nil
}

func (c *DirChecker) GenerateDirHash(path string) (string, error) {
	hasher := sha256.New()

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(path, filePath)

		hasher.Write([]byte(fmt.Sprintf("%s-%d", relPath, info.ModTime().Unix())))
		return nil
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func (c *DirChecker) GetDriHash(path string) string {
	hashes, err := c.getExistingHashes()
	if err != nil {
		return ""
	}

	for _, hash := range hashes {
		if hash.Directory == path {
			return hash.Hash
		}
	}

	return ""
}

func (c *DirChecker) getExistingHashes() ([]*HashEntry, error) {
	if len(c.existingHashes) > 0 {
		return c.existingHashes, nil
	}

	hashFile, err := os.OpenFile(hashFileName, os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer hashFile.Close()

	decoder := json.NewDecoder(hashFile)
	err = decoder.Decode(&c.existingHashes)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return c.existingHashes, nil
}
