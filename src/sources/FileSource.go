package sources

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"os"
	"path"
)

type FileSource struct {
	BaseSource
	watcher *fsnotify.Watcher
	done    chan bool
}

func NewFileSource(log *zap.SugaredLogger, root string) (*FileSource, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileSource{
		BaseSource: BaseSource{
			log:  log,
			root: root,
		},
		watcher: watcher,
		done:    make(chan bool, 1),
	}, nil
}

func (s *FileSource) convertResourceType(e os.FileInfo, path string) Resource {
	var result Resource

	if e.IsDir() {
		result.Type = Directory
	} else {
		result.Type = File
	}
	result.Name = e.Name()
	result.Path = fmt.Sprintf("%v/%v", path, result.Name)
	result.Size = uint32(e.Size())
	result.Mtime = e.ModTime()

	return result
}

func (s *FileSource) ReadDir(path string) ([]Resource, error) {
	ents, err := os.ReadDir(s.absPath(path))
	if err != nil {
		return nil, err
	}

	result := make([]Resource, len(ents))
	for i, e := range ents {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		result[i] = s.convertResourceType(info, path)
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
				if e.Op.Has(fsnotify.Remove) {
					result <- FileEvent{
						Action: Delete,
						Resource: Resource{
							Name: path.Base(e.Name),
							Path: s.relPath(e.Name),
						},
					}
					continue
				}

				stat, err := os.Stat(e.Name)
				if err != nil {
					s.log.Errorf("error stating %v: %v", e.Name, err)
					continue
				}

				fe := FileEvent{
					Action:   Create,
					Resource: s.convertResourceType(stat, path.Dir(e.Name)),
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
