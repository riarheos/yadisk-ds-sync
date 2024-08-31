package filesource

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"path/filepath"
)

type nodeType int

const (
	dirNode nodeType = iota
	fileNode
)

type TreeNode struct {
	Name     string
	Type     nodeType
	Size     int64
	Children []*TreeNode
}

func (t *TreeNode) String() string {
	if t.Type == dirNode {
		return fmt.Sprintf("[d] %s", t.Name)
	}
	return fmt.Sprintf("[f] %s (%d)", t.Name, t.Size)
}

func (t *TreeNode) dump(log *zap.SugaredLogger, pad string) {
	log.Debugf("%s%v", pad, t)
	if t.Type == dirNode {
		for _, child := range t.Children {
			child.dump(log, pad+"  ")
		}
	}
}

type diffElement struct {
	Name string
	Type nodeType
}

func (t *TreeNode) Compare(other *TreeNode) ([]diffElement, error) {
	diff := make([]diffElement, 0)
	return diff, t.compare(other, &diff, "")
}

func (t *TreeNode) compare(other *TreeNode, diff *[]diffElement, path string) error {
	if t.Type != other.Type {
		return errors.New("different type")
	}

	if t.Type == fileNode {
		if other.Size > t.Size {
			*diff = append(*diff, diffElement{filepath.Join(path, t.Name), fileNode})
		}
		return nil
	}

	// at this point both are dirNodes
	mine := treeNodeMap(&t.Children)
	others := treeNodeMap(&other.Children)
	for o, oval := range others {
		m, ok := mine[o]
		if ok {
			if err := m.compare(oval, diff, filepath.Join(path, t.Name)); err != nil {
				return err
			}
		} else {
			treeNodeList(oval, diff, filepath.Join(path, t.Name))
		}
	}

	return nil
}

func treeNodeList(n *TreeNode, diff *[]diffElement, path string) {
	*diff = append(*diff, diffElement{filepath.Join(path, n.Name), n.Type})
	if n.Type == dirNode {
		for _, child := range n.Children {
			treeNodeList(child, diff, filepath.Join(path, n.Name))
		}
	}
}

func treeNodeMap(n *[]*TreeNode) map[string]*TreeNode {
	r := make(map[string]*TreeNode)
	for _, node := range *n {
		r[node.Name] = node
	}
	return r
}
