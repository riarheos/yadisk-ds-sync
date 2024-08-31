package filesource

import "io"

type FileSource interface {
	Tree() (*TreeNode, error)

	MkDir(path string) error
	ReadFile(path string) (io.ReadCloser, error)
	WriteFile(path string, content io.Reader) error
}
