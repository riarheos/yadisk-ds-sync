package filesource

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type nodeType int

const (
	DirNode nodeType = iota
	FileNode
)

type TreeNode struct {
	Name     string      `yaml:"name"`
	Type     nodeType    `yaml:"type"`
	Size     int64       `yaml:"size"`
	Children []*TreeNode `yaml:"children"`
}

func (t *TreeNode) String() string {
	if t.Type == DirNode {
		return fmt.Sprintf("[d] %s", t.Name)
	}
	return fmt.Sprintf("[f] %s (%d)", t.Name, t.Size)
}

func (t *TreeNode) Dump(log *zap.SugaredLogger, pad string) {
	log.Debugf("%s%v", pad, t)
	if t.Type == DirNode {
		for _, child := range t.Children {
			child.Dump(log, pad+"  ")
		}
	}
}

func (t *TreeNode) DumpToFile(log *zap.SugaredLogger, filename string) error {
	log.Infof("Dumping tree to %s", filename)
	b, err := yaml.Marshal(&t)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, b, 0o644)
}

type DiffElement struct {
	Name     string
	Type     nodeType
	IsUpdate bool
}

func (d *DiffElement) String() string {
	if d.Type == DirNode {
		return fmt.Sprintf("[d] %s", d.Name)
	}
	return fmt.Sprintf("[f] %s", d.Name)
}

func (t *TreeNode) Compare(other *TreeNode) ([]DiffElement, error) {
	diff := make([]DiffElement, 0)
	return diff, t.compare(other, &diff, "")
}

func (t *TreeNode) compare(other *TreeNode, diff *[]DiffElement, path string) error {
	if t.Type != other.Type {
		return errors.New("different type")
	}

	if t.Type == FileNode {
		if other.Size > t.Size {
			*diff = append(*diff, DiffElement{filepath.Join(path, t.Name), FileNode, true})
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

func treeNodeList(n *TreeNode, diff *[]DiffElement, path string) {
	*diff = append(*diff, DiffElement{filepath.Join(path, n.Name), n.Type, false})
	if n.Type == DirNode {
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
