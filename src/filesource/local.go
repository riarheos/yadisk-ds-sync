package filesource

import (
	"go.uber.org/zap"
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

func (l *Local) tree(path string) (*TreeNode, error) {
	ents, err := os.ReadDir(filepath.Join(l.path, path))
	if err != nil {
		return nil, err
	}

	t := &TreeNode{
		Name:     path,
		Type:     dirNode,
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
				Type: fileNode,
				Size: fi.Size(),
			}
			t.Children = append(t.Children, child)
		}
	}

	return t, nil
}
