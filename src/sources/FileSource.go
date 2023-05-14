package sources

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"os"
	"strings"
)

type FileSource struct {
	log     *zap.SugaredLogger
	root    string
	watcher *fsnotify.Watcher
	done    chan bool
}

func NewFileSource(log *zap.SugaredLogger, root string) (*FileSource, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileSource{
		log:     log,
		root:    root,
		watcher: watcher,
		done:    make(chan bool, 1),
	}, nil
}

func (s *FileSource) ReadDir(path string) ([]Resource, error) {
	ents, err := os.ReadDir(s.absPath(path))
	if err != nil {
		return nil, err
	}

	result := make([]Resource, len(ents))
	for i, e := range ents {
		if e.IsDir() {
			result[i].Type = Directory
		} else {
			result[i].Type = File
		}
		result[i].Name = e.Name()
		result[i].Path = fmt.Sprintf("%v/%v", path, result[i].Name)

		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		result[i].Size = uint32(info.Size())
		result[i].Mtime = info.ModTime()
	}

	return result, nil
}

func (s *FileSource) ReadFile(path string) (io.ReadCloser, error) {
	return os.Open(s.absPath(path))
}

func (s *FileSource) WriteFile(path string) (io.WriteCloser, error) {
	return os.Create(s.absPath(path))
}

func (s *FileSource) Mkdir(path string) error {
	return os.Mkdir(s.absPath(path), fs.ModeDir|0755)
}

func (s *FileSource) AwaitIO() error {
	return nil
}

func (s *FileSource) Destroy() error {
	s.done <- true
	return s.watcher.Close()
}

func (s *FileSource) WatchDir(path string) error {
	return s.watcher.Add(s.absPath(path))
}

func (s *FileSource) Events() chan FileEvent {
	result := make(chan FileEvent)

	go func() {

		for {
			select {
			case e := <-s.watcher.Events:
				fe := FileEvent{}
				fe.Name = s.relPath(e.Name)
				fe.Path = e.Name

				if e.Op.Has(fsnotify.Remove) {
					fe.Action = Delete
					result <- fe
					continue
				}

				fe.Action = Create

				stat, err := os.Stat(e.Name)
				if err != nil {
					s.log.Errorf("error stating %v: %v", e.Name, err)
					continue
				}

				if stat.IsDir() {
					fe.Type = Directory
				} else {
					fe.Type = File
				}

				if fe.Type == File {
					fe.Size = uint32(stat.Size())
				}

				s.log.Debugf("Found new diffsync file %v (%v)", fe.Path, fe.Mtime)
				result <- fe
			case e := <-s.watcher.Errors:
				s.log.Errorf("error watching: %v", e)
			case <-s.done:
				return
			}
		}

	}()

	return result
}

func (s *FileSource) absPath(path string) string {
	if path == "" {
		return s.root
	}
	return fmt.Sprintf("%v%v", s.root, path)
}

func (s *FileSource) relPath(path string) string {
	return strings.Replace(path, s.root, "", 1)
}
