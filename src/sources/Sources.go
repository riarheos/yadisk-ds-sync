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
	// ReadDir scans the directory and returns files and subdirs
	ReadDir(path string) ([]Resource, error)

	// ReadFile initiates a file read operation. May use other goroutines, see Await()
	ReadFile(path string) (io.ReadCloser, error)

	// WriteFile initiates a file write operation. May use other goroutines, see Await()
	WriteFile(path string) (io.WriteCloser, error)

	// Mkdir creates an empty directory. Sync only.
	Mkdir(path string) error

	// Await waits for the operation to complete. If ReadFile() or WriteFile() initiated
	// a goroutine, then this method is to wait for it to complete.
	Await() error
}
