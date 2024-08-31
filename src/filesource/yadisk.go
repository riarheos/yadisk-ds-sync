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

type resourceHref struct {
	HREF string `json:"href"`
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

func (y *Yadisk) Tree() (*TreeNode, error) {
	y.log.Info("Gathering yadisk file info")
	return y.tree("")
}

func (y *Yadisk) MkDir(path string) error {
	y.log.Infof("Creating directory %s", path)
	uri := fmt.Sprintf("%s/resources?path=%s", api, url.QueryEscape(filepath.Join(y.cfg.Path, path)))
	_, err := y.http(uri, "PUT")
	return err
}

func (y *Yadisk) ReadFile(path string) (io.ReadCloser, error) {
	y.log.Infof("Downloading file %s", path)

	var b []byte
	var err error
	uri := fmt.Sprintf("%s/resources/download?path=%s", api, url.QueryEscape(filepath.Join(y.cfg.Path, path)))
	if b, err = y.http(uri, "GET"); err != nil {
		return nil, err
	}

	var dr resourceHref
	if err = json.Unmarshal(b, &dr); err != nil {
		return nil, err
	}

	resp, err := y.cli.Get(dr.HREF)
	if err != nil {
		return nil, err
	}
	if !isSuccess(resp.StatusCode) {
		return nil, fmt.Errorf("got http error: %v", resp.Status)
	}

	return resp.Body, nil
}

func (y *Yadisk) WriteFile(path string, content io.Reader) error {
	y.log.Infof("Uploading file %s", path)

	var b []byte
	var err error
	uri := fmt.Sprintf("%s/resources/upload?overwrite=true&path=%s", api, url.QueryEscape(filepath.Join(y.cfg.Path, path)))
	if b, err = y.http(uri, "GET"); err != nil {
		return err
	}

	var dr resourceHref
	if err = json.Unmarshal(b, &dr); err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", dr.HREF, content)
	if err != nil {
		return err
	}

	resp, err := y.cli.Do(req)
	if err != nil {
		return err
	}
	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("got http error: %v", resp.Status)
	}

	return nil
}

func (y *Yadisk) getOneDir(path string) (*yadiskNode, error) {
	y.log.Debugf("Getting directory %s", path)

	var b []byte
	var err error
	uri := fmt.Sprintf("%s/resources?path=%s", api, url.QueryEscape(filepath.Join(y.cfg.Path, path)))
	if b, err = y.http(uri, "GET"); err != nil {
		return nil, err
	}

	var lr yadiskNode
	if err = json.Unmarshal(b, &lr); err != nil {
		return nil, err
	}

	return &lr, nil
}

func (y *Yadisk) http(path string, method string) ([]byte, error) {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("OAuth %s", y.cfg.Token))

	resp, err := y.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if !isSuccess(resp.StatusCode) {
		return nil, fmt.Errorf("got http error: %v", resp.Status)
	}

	return io.ReadAll(resp.Body)
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

func isSuccess(code int) bool {
	return code >= 200 && code < 300
}
