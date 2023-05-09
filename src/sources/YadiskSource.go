package sources

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"time"
)

type YadiskSource struct {
	log        *zap.SugaredLogger
	oauth      string
	client     http.Client
	slowClient http.Client
	root       string
}

type YadiskResource struct {
	Name  string    `json:"name"`
	Path  string    `json:"path"`
	Type  string    `json:"type"`
	Size  uint32    `json:"size"`
	Mtime time.Time `json:"modified"`
}

type yadiskResourceResponse struct {
	Embedded struct {
		Items []YadiskResource `json:"items"`
	} `json:"_embedded"`
}

func NewYadiskSource(log *zap.SugaredLogger, token string, root string) *YadiskSource {
	return &YadiskSource{
		log:   log,
		oauth: fmt.Sprintf("OAuth %v", token),
		client: http.Client{
			Timeout: 5 * time.Second,
		},
		slowClient: http.Client{
			Timeout: 300 * time.Second,
		},
		root: "disk:/" + root,
	}
}

func (s *YadiskSource) get(url string, result interface{}) error {
	fullUrl := fmt.Sprintf("https://cloud-api.yandex.net/v1/disk/%v", url)
	s.log.Debugf("GET %v", fullUrl)
	req, err := http.NewRequest("GET", fullUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", s.oauth)

	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return fmt.Errorf("status %v %v while getting %v", res.StatusCode, res.Status, fullUrl)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, result)
	if err != nil {
		return err
	}

	return nil
}

func (s *YadiskSource) ReadDir(path string) ([]Resource, error) {
	q := url.Values{}
	q.Add("limit", "10000")
	q.Add("path", s.AbsPath(path))

	var res yadiskResourceResponse
	err := s.get(fmt.Sprintf("resources?%v", q.Encode()), &res)
	if err != nil {
		return nil, err
	}

	result := make([]Resource, len(res.Embedded.Items))
	for i, e := range res.Embedded.Items {
		if e.Type == "dir" {
			result[i].Type = Directory
		} else {
			result[i].Type = File
		}
		result[i].Name = e.Name
		result[i].Path = e.Path

		result[i].Size = e.Size
		result[i].Mtime = e.Mtime
	}

	return result, nil
}

func (s *YadiskSource) ReadFile(path string) (io.ReadCloser, error) {
	q := url.Values{}
	q.Add("path", s.AbsPath(path))

	var downloadRes struct {
		Href string `json:"href"`
	}
	err := s.get(fmt.Sprintf("resources/download?%v", q.Encode()), &downloadRes)
	if err != nil {
		return nil, err
	}

	s.log.Debugf("Downloading %v via %v", path, downloadRes.Href)
	req, err := http.NewRequest("GET", downloadRes.Href, nil)
	if err != nil {
		return nil, err
	}
	res, err := s.slowClient.Do(req)
	if err != nil {
		_ = res.Body.Close()
		return nil, err
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		_ = res.Body.Close()
		return nil, fmt.Errorf("download status: %v %v", res.StatusCode, res.Status)
	}

	return res.Body, nil
}

func (s *YadiskSource) WriteFile(path string) (io.WriteCloser, error) {
	q := url.Values{}
	q.Add("path", s.AbsPath(path))

	var uploadRes struct {
		Href string `json:"href"`
	}
	err := s.get(fmt.Sprintf("resources/upload?%v", q.Encode()), &uploadRes)
	if err != nil {
		return nil, err
	}

	s.log.Debugf("Uploading %v via %v", path, uploadRes.Href)
	reader, writer := io.Pipe()

	req, err := http.NewRequest("PUT", uploadRes.Href, reader)
	if err != nil {
		_ = writer.Close()
		return nil, err
	}

	go func() {
		res, err := s.slowClient.Do(req)
		if err != nil {
			_ = writer.Close()
			_ = res.Body.Close()
		}
	}()

	return writer, nil
}

func (s *YadiskSource) AbsPath(path string) string {
	if path == "" {
		return s.root
	}
	if len(path) > 0 && path[0] == '/' {
		return fmt.Sprintf("%v%v", s.root, path)
	}
	return fmt.Sprintf("%v/%v", s.root, path)
}
