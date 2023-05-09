package synca

import (
	"fmt"
	"go.uber.org/zap"
	"io"
	"yadisk-ds-sync/src/sources"
)

type Synca struct {
	log *zap.SugaredLogger
	src []sources.GenericSource
}

func New(log *zap.SugaredLogger, src []sources.SyncSource, token string) (*Synca, error) {
	log.Infof("Creating a synchronization from '%v' to '%v'", src[0].URL(), src[1].URL())

	if len(src) != 2 {
		return nil, fmt.Errorf("can only work with two sources")
	}

	result := Synca{
		log: log,
	}

	for _, s := range src {
		switch s.Type {
		case "file":
			result.src = append(result.src, sources.NewFileSource(log, s.Root))
		case "yadisk":
			result.src = append(result.src, sources.NewYadiskSource(log, token, s.Root))
		default:
			return nil, fmt.Errorf("unknown source type %v", s.Type)
		}
	}

	return &result, nil
}

func (s *Synca) Run() error {
	return s.sync("")
}

func findMissingItems(left map[string]sources.Resource, right map[string]sources.Resource) []string {
	var result []string

	for l := range left {
		_, ok := right[l]
		if !ok {
			result = append(result, l)
		}
	}

	return result
}

func findResources(src sources.GenericSource, path string) (map[string]sources.Resource, error) {
	resources, err := src.ReadDir(path)
	if err != nil {
		return nil, err
	}
	rmap := make(map[string]sources.Resource)
	for _, l := range resources {
		rmap[l.Name] = l
	}
	return rmap, nil
}

func (s *Synca) sync(relativePath string) error {
	lm, err := findResources(s.src[0], relativePath)
	if err != nil {
		s.log.Errorf("cannot lookup resources")
		return err
	}

	rm, err := findResources(s.src[1], relativePath)
	if err != nil {
		s.log.Errorf("cannot lookup resources")
		return err
	}

	for _, mi := range findMissingItems(rm, lm) {
		s.log.Infof("Missing resource %v", s.src[1].AbsPath(rm[mi].Path))

		reader, err := s.src[1].ReadFile(rm[mi].Path)
		if err != nil {
			s.log.Errorf("error reading %v: %v", s.src[1].AbsPath(rm[mi].Path), err)
			continue
		}

		writer, err := s.src[0].WriteFile(rm[mi].Path)
		if err != nil {
			_ = reader.Close()
			s.log.Errorf("error writing %v: %v", s.src[0].AbsPath(rm[mi].Path), err)
			continue
		}

		bytes, err := io.Copy(writer, reader)
		_ = reader.Close()
		_ = writer.Close()
		if err != nil {
			s.log.Errorf("error copying %v: %v", s.src[1].AbsPath(rm[mi].Path), err)
		}
		s.log.Infof("Copied %v bytes", bytes)
	}

	return nil
}
