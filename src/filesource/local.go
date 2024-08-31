package filesource

import (
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
)

type LocalConfig struct {
	Path string `yaml:"path"`
}

type Local struct {
	log  *zap.SugaredLogger
	path string
}

func NewLocal(log *zap.SugaredLogger, cfg *LocalConfig) *Local {
	return &Local{
		log:  log,
		path: cfg.Path,
	}
}

func (l *Local) Tree() (*TreeNode, error) {
	l.log.Info("Gathering local file info")
	return l.tree("")
}

func (l *Local) MkDir(path string) error {
	l.log.Infof("Creating directory %s", path)
	return os.Mkdir(filepath.Join(l.path, path), 0755)
}

func (l *Local) ReadFile(path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.path, path))
}

func (l *Local) WriteFile(path string, content io.Reader) error {
	file, err := os.OpenFile(filepath.Join(l.path, path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = io.Copy(file, content)
	if err != nil {
		return err
	}

	return nil
}

func (l *Local) tree(path string) (*TreeNode, error) {
	ents, err := os.ReadDir(filepath.Join(l.path, path))
	if err != nil {
		return nil, err
	}

	t := &TreeNode{
		Name:     path,
		Type:     DirNode,
		Children: make([]*TreeNode, 0),
	}

	for _, ent := range ents {
		if ent.IsDir() {
			sub, err := l.tree(filepath.Join(path, ent.Name()))
			if err != nil {
				return nil, err
			}
			t.Children = append(t.Children, sub)
		} else {
			fi, err := ent.Info()
			if err != nil {
				return nil, err
			}
			child := &TreeNode{
				Name: ent.Name(),
				Type: FileNode,
				Size: fi.Size(),
			}
			t.Children = append(t.Children, child)
		}
	}

	return t, nil
}
