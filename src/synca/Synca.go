package synca

import (
	"fmt"
	"go.uber.org/zap"
	"io"
	"strings"
	"sync"
	"yadisk-ds-sync/src/sources"
)

type treeNode struct {
	dirs     map[string]*treeNode
	files    map[string]sources.Resource
	selfPath string // see comments in sources.Resource
}

type Synca struct {
	log   *zap.SugaredLogger
	src   []sources.GenericSource
	trees []*treeNode
	Done  chan bool
}

func New(log *zap.SugaredLogger, src []sources.SyncSource, token string) (*Synca, error) {
	result := Synca{
		log:  log,
		Done: make(chan bool, 1),
	}

	for _, s := range src {
		log.Infof("Registering sync source '%v'", s.URL())
		switch s.Type {
		case "file":
			fs, err := sources.NewFileSource(log, s.Root)
			if err != nil {
				return nil, err
			}
			result.src = append(result.src, fs)
		case "yadisk":
			result.src = append(result.src, sources.NewYadiskSource(log, token, s.Root))
		default:
			return nil, fmt.Errorf("unknown source type %v", s.Type)
		}
	}

	return &result, nil
}

func (s *Synca) Destroy() error {
	var err error = nil
	for _, s := range s.src {
		e := s.Destroy()
		if e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (s *Synca) Run() error {
	s.trees = make([]*treeNode, len(s.src))

	// prepare trees
	s.log.Info("Preparing trees")
	for i, ss := range s.src {
		t, err := s.getTree(ss, "")
		if err != nil {
			return err
		}
		s.trees[i] = t
	}

	// initial sync
	s.log.Info("Doing initial sync")
	err := s.syncTrees(s.src[0], s.src[1], s.trees[0], s.trees[1])
	if err != nil {
		return err
	}
	err = s.syncTrees(s.src[1], s.src[0], s.trees[1], s.trees[0])
	if err != nil {
		return err
	}
	s.log.Info("Initial sync done")

	var wg sync.WaitGroup
	wg.Add(len(s.src))

	for _, src := range s.src {
		go s.diffSync(src, &wg)
	}

	wg.Wait()
	s.log.Info("Sync terminating")
	return nil
}

func (s *Synca) diffSync(src sources.GenericSource, wg *sync.WaitGroup) {
	dst := s.getOtherSource(src)
	evts := src.Events()
outer:
	for {
		var e sources.FileEvent

		select {
		case <-s.Done:
			s.Done <- true // set done flag for other sources
			break outer
		case e = <-evts:
		}

		// this goes first to be quick enough to see the file creation for 'mkdir && touch' ops
		if e.Action == sources.Create && e.Type == sources.Directory {
			err := src.WatchDir(e.Name)
			if err != nil {
				s.log.Errorf("error installing watcher to %v: %v", e.Name, err)
			}
		}

		if e.Action == sources.Create {
			if e.Type == sources.File {
				s.log.Infof("Syncing new file: %v", e.Name)
				err := s.copySingleFile(src, dst, e.Name)
				if err != nil {
					s.log.Errorf("Error copying %v: %v", e.Name, err)
				}
			} else {
				s.log.Infof("Syncing new dir: %v", e.Name)
				err := dst.Mkdir(e.Path)
				if err != nil {
					s.log.Errorf("Error making dir %v: %v", e.Name, err)
				}
			}
		} else {
			s.log.Warnf("Got a delete for %v, ignoring it", e.Name)
		}
	}

	wg.Done()
}

// getTree fetches recursively a tree from the source. No data is modified.
// However, watchers are installed on directories along the descent.
func (s *Synca) getTree(src sources.GenericSource, path string) (*treeNode, error) {
	items, err := src.ReadDir(path)
	if err != nil {
		return nil, err
	}

	err = src.WatchDir(path)
	if err != nil {
		return nil, err
	}

	node := treeNode{
		dirs:     make(map[string]*treeNode),
		files:    make(map[string]sources.Resource),
		selfPath: path,
	}

	for _, i := range items {
		switch i.Type {
		case sources.File:
			node.files[i.Name] = i
		case sources.Directory:
			subnode, err := s.getTree(src, fmt.Sprintf("%v/%v", path, i.Name))
			if err != nil {
				return nil, err
			}
			node.dirs[i.Name] = subnode
		default:
			return nil, fmt.Errorf("unknown node type %v", i.Type)
		}
	}

	return &node, nil
}

// syncTrees tries to recursively sync two trees. It will modify file contents if required.
func (s *Synca) syncTrees(src sources.GenericSource, dst sources.GenericSource, srcTree *treeNode, dstTree *treeNode) error {
	for fileName, file := range srcTree.files {
		_, ok := dstTree.files[fileName]
		if !ok {
			s.log.Infof("file %v is missing", file.Path)
			err := s.copySingleFile(src, dst, file.Path)
			if err != nil {
				return err
			}
		}
	}

	for dirName, dir := range srcTree.dirs {
		destDir, ok := dstTree.dirs[dirName]
		if !ok {
			s.log.Infof("dir %v is missing", dir.selfPath)
			err := dst.Mkdir(dir.selfPath)
			if err != nil {
				return err
			}
			destDir = &treeNode{
				dirs:     make(map[string]*treeNode),
				files:    make(map[string]sources.Resource),
				selfPath: dir.selfPath,
			}
		}

		err := s.syncTrees(src, dst, dir, destDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Synca) copySingleFile(src sources.GenericSource, dst sources.GenericSource, filePath string) error {
	reader, err := src.ReadFile(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	writer, err := dst.WriteFile(filePath)
	if err != nil {
		return err
	}
	defer writer.Close()

	bytes, err := io.Copy(writer, reader)
	if err != nil {
		return err
	}

	s.log.Debugf("Copied %v bytes", bytes)
	reader.Close()
	writer.Close()

	if err := src.AwaitIO(); err != nil {
		return err
	}
	if err := dst.AwaitIO(); err != nil {
		return err
	}
	return nil
}

func (s *Synca) getOtherSource(src sources.GenericSource) sources.GenericSource {
	if s.src[0] == src {
		return s.src[1]
	} else {
		return s.src[0]
	}
}

func (t *treeNode) dump(pad int) {
	padding := strings.Repeat(" +  ", pad)

	for _, f := range t.files {
		fmt.Printf("%v%v (size=%v, mtime=%v, path=%v)\n", padding, f.Name, f.Size, f.Mtime, f.Path)
	}
	for n, d := range t.dirs {
		fmt.Printf("%v[DIR] %v (path=%v)\n", padding, n, d.selfPath)
		d.dump(pad + 1)
	}
}
