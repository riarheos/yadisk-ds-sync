package sources

import (
	"fmt"
	"go.uber.org/zap"
	"io"
	"strings"
	"time"
)

const (
	Directory = iota + 1
	File
)

type Resource struct {
	Type uint8

	Name string // human-readable, just the file name
	Path string // path relative to the source root, will have a slash in the beginning due to the creation rules

	// only set for Type == file
	Size  uint32
	Mtime time.Time
}

type SyncSource struct {
	Type string `yaml:"type"`
	Root string `yaml:"root"`
}

const (
	Create = iota + 1
	Delete
)

type FileEvent struct {
	Resource
	Action uint8
}

func (ss *SyncSource) URL() string {
	return fmt.Sprintf("%v://%v", ss.Type, ss.Root)
}

type GenericSource interface {
	// ***
	// *** Global routines
	// ***

	// Destroy is a global destructor
	Destroy() error

	// ***
	// *** File Access Routines
	// ***

	// ReadDir scans the directory and returns files and subdirs
	ReadDir(path string) ([]Resource, error)

	// ReadFile initiates a file read operation. May use other goroutines, see Await()
	ReadFile(path string) (io.ReadCloser, error)

	// WriteFile initiates a file write operation. May use other goroutines, see Await()
	WriteFile(path string) (io.WriteCloser, error)

	// Mkdir creates an empty directory. Sync only.
	Mkdir(path string) error

	// AwaitIO waits for the operation to complete. If ReadFile() or WriteFile() initiated
	// a goroutine, then this method is to wait for it to complete.
	AwaitIO() error

	// ***
	// *** Diff IO Routines
	// ***

	// WatchDir initiates a diff watcher for the specified directory
	WatchDir(path string) error

	// Events returns a channel with relative file names that have changed
	Events() chan FileEvent
}

type BaseSource struct {
	log  *zap.SugaredLogger
	root string
}

func (s *BaseSource) absPath(path string) string {
	if path == "" {
		return s.root
	}
	return fmt.Sprintf("%v%v", s.root, path)
}

func (s *BaseSource) relPath(path string) string {
	return strings.Replace(path, s.root, "", 1)
}
