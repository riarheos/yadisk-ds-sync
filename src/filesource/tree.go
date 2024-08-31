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

func NewTreeNode(filename string) (*TreeNode, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	t := &TreeNode{}
	err = yaml.Unmarshal(data, t)
	if err != nil {
		return nil, err
	}

	return t, nil
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
	Name string
	Type nodeType
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

// compare adds to the diff every node present in **t** that is not present in **other**
func (t *TreeNode) compare(other *TreeNode, diff *[]DiffElement, path string) error {
	ownPath := filepath.Join(path, t.Name)

	if other == nil {
		*diff = append(*diff, DiffElement{ownPath, t.Type})
		for _, child := range t.Children {
			if err := child.compare(nil, diff, ownPath); err != nil {
				return err
			}
		}
		return nil
	}

	if t.Type != other.Type {
		return errors.New("different type")
	}

	if t.Type == FileNode {
		// for now consider the bigger file more recent
		// TODO: use some mtime logic maybe?
		if t.Size > other.Size {
			*diff = append(*diff, DiffElement{ownPath, FileNode})
		}
		return nil
	}

	// at this point both are dirNodes
	others := treeNodeMap(&other.Children)
	for _, child := range t.Children {
		otherChild, _ := others[child.Name]
		if err := child.compare(otherChild, diff, ownPath); err != nil {
			return err
		}
	}

	return nil
}

func treeNodeMap(n *[]*TreeNode) map[string]*TreeNode {
	r := make(map[string]*TreeNode)
	for _, node := range *n {
		r[node.Name] = node
	}
	return r
}
