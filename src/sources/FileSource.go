package sources

import (
	"fmt"
	"go.uber.org/zap"
	"io"
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
		result[i].Path = fmt.Sprintf("%v/%v", path, e.Name())
	}

	return result, nil
}

func (s *FileSource) ReadFile(path string) (io.ReadCloser, error) {
	return os.Open(s.AbsPath(path))
}

func (s *FileSource) WriteFile(path string) (io.WriteCloser, error) {
	return os.Create(s.AbsPath(path))
}

func (s *FileSource) AbsPath(path string) string {
	if len(path) > 0 && path[0] == '/' {
		return fmt.Sprintf("%v%v", s.root, path)
	}
	return fmt.Sprintf("%v/%v", s.root, path)
}
