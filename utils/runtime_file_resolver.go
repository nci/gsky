package utils

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type RuntimeFileResolver struct {
	DataDirs   []string
	fileLookup map[string]string
}

func NewRuntimeFileResolver(searchPath string) *RuntimeFileResolver {
	resolver := &RuntimeFileResolver{
		fileLookup: make(map[string]string),
	}

	searchPathList := strings.Split(searchPath, ":")
	for _, dataDir := range searchPathList {
		dataDir = strings.TrimSpace(dataDir)
		if len(dataDir) == 0 {
			continue
		}
		resolver.DataDirs = append(resolver.DataDirs, dataDir)
	}

	cwd, err := os.Getwd()
	if err == nil {
		resolver.DataDirs = append(resolver.DataDirs, cwd)
	} else {
		log.Printf("Failed to get CWD: %v", err)
	}

	executablePath := filepath.Dir(os.Args[0])
	resolver.DataDirs = append(resolver.DataDirs, executablePath)
	return resolver
}

func (r *RuntimeFileResolver) Resolve(filePath string) (string, error) {
	if strings.HasPrefix(filePath, "/") {
		err := checkFile(filePath)
		return filePath, err
	}

	for _, dataDir := range r.DataDirs {
		path := path.Clean(path.Join(dataDir, filePath))
		err := checkFile(path)
		if err == nil {
			return path, nil
		}
	}

	return filePath, fmt.Errorf("Failed to resolve %v", filePath)
}

func (r *RuntimeFileResolver) Lookup(filePath string) (string, error) {
	if path, found := r.fileLookup[filePath]; found {
		return path, nil
	}

	path, err := r.Resolve(filePath)
	if err != nil {
		return "", err
	}
	r.fileLookup[filePath] = path
	return path, nil
}

func checkFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}
	return nil
}
