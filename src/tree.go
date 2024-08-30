package main

import (
	"fmt"
	"go.uber.org/zap"
)

type nodeType int

const (
	dirNode nodeType = iota
	fileNode
)

type treeNode struct {
	Name     string
	Type     nodeType
	Size     int64
	Children []*treeNode
}

func (t *treeNode) String() string {
	if t.Type == dirNode {
		return fmt.Sprintf("[d] %s", t.Name)
	}
	return fmt.Sprintf("[f] %s (%d)", t.Name, t.Size)
}

func (t *treeNode) dump(log *zap.SugaredLogger, pad string) {
	log.Debugf("%s%v", pad, t)
	if t.Type == dirNode {
		for _, child := range t.Children {
			child.dump(log, pad+"  ")
		}
	}
}
