package sources

import (
	"fmt"
	"io"
	"time"
)

const (
	Directory = "dir"
	File      = "file"
)

type Resource struct {
	Type string

	Name string // human readable, just the file name
	Path string // path relative to the source root, will have a slash in the beginning due to the creation rules

	// only set for Type == file
	Size  uint32
	Mtime time.Time
}

type SyncSource struct {
	Type string `yaml:"type"`
	Root string `yaml:"root"`
}

func (ss *SyncSource) URL() string {
	return fmt.Sprintf("%v://%v", ss.Type, ss.Root)
}

type GenericSource interface {
	ReadDir(path string) ([]Resource, error)
	ReadFile(path string) (io.ReadCloser, error)
	WriteFile(path string) (io.WriteCloser, error)
	AbsPath(path string) string
}
