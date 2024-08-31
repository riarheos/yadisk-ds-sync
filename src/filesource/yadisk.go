package filesource

import (
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

const api = "https://cloud-api.yandex.net/v1/disk"

type YadiskConfig struct {
	Path    string        `yaml:"path"`
	Token   string        `yaml:"token"`
	Timeout time.Duration `yaml:"timeout"`
}

type Yadisk struct {
	log *zap.SugaredLogger
	cfg *YadiskConfig

	tr  *http.Transport
	cli *http.Client
}

func NewYadisk(log *zap.SugaredLogger, cfg *YadiskConfig) *Yadisk {
	tr := &http.Transport{
		MaxIdleConns:    5,
		IdleConnTimeout: 30 * time.Second,
	}

	y := &Yadisk{
		log: log,
		cfg: cfg,
		tr:  tr,
		cli: &http.Client{
			Transport: tr,
			Timeout:   cfg.Timeout,
		},
	}

	return y
}

type yadiskNode struct {
	Name     string `json:"name"`
	Type     string `json:"type"`    // "dir" or "file"
	Path     string `json:"path"`    // "disk:/DND/screen/dirname"
	Created  string `json:"created"` // "2020-05-12T22:14:47+00:00"
	Modified string `json:"modified"`

	// only for type == "file"
	Size int64 `json:"size"`

	// only for type == "dir"
	Embedded struct {
		Items []yadiskNode `json:"items"`
	} `json:"_embedded"`
}

func (y *Yadisk) getOneDir(path string) (*yadiskNode, error) {
	y.log.Debugf("Getting directory %s", path)

	var b []byte
	var err error
	if b, err = y.http(
		fmt.Sprintf("%s/resources?path=%s", api, url.QueryEscape(filepath.Join(y.cfg.Path, path))),
	); err != nil {
		return nil, err
	}

	var lr yadiskNode
	if err = json.Unmarshal(b, &lr); err != nil {
		return nil, err
	}

	return &lr, nil
}

func (y *Yadisk) http(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("OAuth %s", y.cfg.Token))

	resp, err := y.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got http error: %v", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (y *Yadisk) Tree() (*TreeNode, error) {
	y.log.Info("Gathering yadisk file info")
	return y.tree("")
}

func (y *Yadisk) tree(path string) (*TreeNode, error) {
	node, err := y.getOneDir(path)
	if err != nil {
		return nil, err
	}
	if node.Type != "dir" {
		return nil, errors.New("not a dir")
	}

	t := &TreeNode{
		Type:     dirNode,
		Name:     node.Name,
		Children: make([]*TreeNode, 0),
	}
	for _, emb := range node.Embedded.Items {
		if emb.Type == "file" {
			sub := &TreeNode{
				Type: fileNode,
				Name: emb.Name,
				Size: emb.Size,
			}
			t.Children = append(t.Children, sub)
		} else {
			sub, err := y.tree(filepath.Join(path, emb.Name))
			if err != nil {
				return nil, err
			}
			t.Children = append(t.Children, sub)
		}
	}

	return t, nil
}
