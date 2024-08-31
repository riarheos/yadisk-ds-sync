package filesource

type FileSource interface {
	Tree() (*TreeNode, error)
}
