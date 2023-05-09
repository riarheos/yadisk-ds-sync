package sources

import (
	"fmt"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"os"
)

type FileSource struct {
	log  *zap.SugaredLogger
	root string
}

func NewFileSource(log *zap.SugaredLogger, root string) *FileSource {
	return &FileSource{
		log:  log,
		root: root,
	}
}

func (s *FileSource) ReadDir(path string) ([]Resource, error) {
	ents, err := os.ReadDir(s.AbsPath(path))
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
	return os.Open(s.AbsPath(path))
}

func (s *FileSource) WriteFile(path string) (io.WriteCloser, error) {
	return os.Create(s.AbsPath(path))
}

func (s *FileSource) Mkdir(path string) error {
	return os.Mkdir(s.AbsPath(path), fs.ModeDir|0755)
}

func (s *FileSource) Await() error {
	return nil
}

func (s *FileSource) AbsPath(path string) string {
	if path == "" {
		return s.root
	}
	return fmt.Sprintf("%v%v", s.root, path)
}
