package main

import (
	"go.uber.org/zap"
	"os"
	"path/filepath"
)

type local struct {
	log  *zap.SugaredLogger
	path string
}

func newLocal(log *zap.SugaredLogger, cfg *localConfig) *local {
	return &local{
		log:  log,
		path: cfg.Path,
	}
}

func (l *local) Tree() (*treeNode, error) {
	l.log.Info("Gathering local file info")
	return l.tree("")
}

func (l *local) tree(path string) (*treeNode, error) {
	ents, err := os.ReadDir(filepath.Join(l.path, path))
	if err != nil {
		return nil, err
	}

	t := &treeNode{
		Name:     path,
		Type:     dirNode,
		Children: make([]*treeNode, 0),
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
			child := &treeNode{
				Name: ent.Name(),
				Type: fileNode,
				Size: fi.Size(),
			}
			t.Children = append(t.Children, child)
		}
	}

	return t, nil
}
