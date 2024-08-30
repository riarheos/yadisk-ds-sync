package sources

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type YadiskSource struct {
	BaseSource
	oauth      string
	client     http.Client
	slowClient http.Client
	mtx        sync.Mutex
	err        error
	lastMtime  time.Time
	done       chan bool
}

type YadiskResource struct {
	Name  string    `json:"name"`
	Path  string    `json:"path"`
	Type  string    `json:"type"`
	Size  uint32    `json:"size"`
	Mtime time.Time `json:"modified"`
}

type yadiskItemsResponse struct {
	Items []YadiskResource `json:"items"`
}

type yadiskResourceResponse struct {
	Embedded yadiskItemsResponse `json:"_embedded"`
}

func NewYadiskSource(log *zap.SugaredLogger, token string, root string) *YadiskSource {
	return &YadiskSource{
		BaseSource: BaseSource{
			log:  log,
			root: "disk:/" + root,
		},
		oauth: fmt.Sprintf("OAuth %v", token),
		client: http.Client{
			Timeout: 10 * time.Second,
		},
		slowClient: http.Client{
			Timeout: 300 * time.Second,
		},
		done: make(chan bool, 1),
	}
}

func (s *YadiskSource) get(url string, result interface{}) error {
	return s.http("GET", url, result)
}

func (s *YadiskSource) put(url string, result interface{}) error {
	return s.http("PUT", url, result)
}

func (s *YadiskSource) http(method string, url string, result interface{}) error {
	fullUrl := fmt.Sprintf("https://cloud-api.yandex.net/v1/disk/%v", url)
	s.log.Debugf("%v %v", method, fullUrl)

	req, err := http.NewRequest(method, fullUrl, nil)
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
		return fmt.Errorf("status %v while getting %v", res.Status, fullUrl)
	}

	s.log.Debugf("status %v", res.Status)

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

func (s *YadiskSource) convertResourceType(e YadiskResource) Resource {
	var result Resource

	if e.Type == "dir" {
		result.Type = Directory
	} else {
		result.Type = File
	}
	result.Name = e.Name
	result.Path = s.relPath(e.Path)
	result.Size = e.Size
	result.Mtime = e.Mtime

	return result
}

func (s *YadiskSource) ReadDir(path string) ([]Resource, error) {
	q := url.Values{}
	q.Add("limit", "10000")
	q.Add("path", s.absPath(path))

	var res yadiskResourceResponse
	err := s.get(fmt.Sprintf("resources?%v", q.Encode()), &res)
	if err != nil {
		return nil, err
	}

	result := make([]Resource, len(res.Embedded.Items))
	for i, e := range res.Embedded.Items {
		result[i] = s.convertResourceType(e)
		if e.Mtime.After(s.lastMtime) {
			s.lastMtime = e.Mtime
		}
	}

	return result, nil
}

func (s *YadiskSource) ReadFile(path string) (io.ReadCloser, error) {
	q := url.Values{}
	q.Add("path", s.absPath(path))

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
	q.Add("path", s.absPath(path))

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

	s.mtx.Lock()
	go func() {
		res, err := s.slowClient.Do(req)
		if err != nil {
			_ = writer.Close()
			_ = res.Body.Close()
			s.err = err
		}
		s.log.Debugf("status %v", res.Status)
		s.mtx.Unlock()
	}()

	return writer, nil
}

func (s *YadiskSource) Mkdir(path string) error {
	q := url.Values{}
	q.Add("path", s.absPath(path))

	var mkdirRes struct {
		Href string `json:"href"`
	}
	err := s.put(fmt.Sprintf("resources?%v", q.Encode()), &mkdirRes)
	if err != nil {
		return err
	}

	return nil
}

func (s *YadiskSource) AwaitIO() error {
	s.mtx.Lock()
	s.mtx.Unlock()
	return s.err
}

func (s *YadiskSource) Destroy() error {
	s.done <- true
	return nil
}

func (s *YadiskSource) WatchDir(_ string) error {
	// here we can simply ignore all the watch requests because
	// we filter out all events globally by the prefix
	return nil
}

func (s *YadiskSource) Events() chan FileEvent {
	result := make(chan FileEvent)

	go func() {

	outer:
		for {
			t := time.NewTimer(15 * time.Second)

			select {
			case <-s.done:
				break outer
			case <-t.C:
				// fall through
			}

			q := url.Values{}
			q.Add("limit", "100")

			var res struct {
				Items []YadiskResource `json:"items"`
			}
			err := s.get(fmt.Sprintf("resources/last-uploaded?%v", q.Encode()), &res)
			if err != nil {
				s.log.Errorf("Could not fetch update-resources: %v", err)
				continue
			}

			var packLastMtime time.Time
			for _, item := range res.Items {
				if strings.HasPrefix(item.Path, s.root) && item.Mtime.After(s.lastMtime) {
					s.log.Debugf("Found new diffsync file %v (%v)", item.Path, item.Mtime)
					r := FileEvent{
						Action:   Create,
						Resource: s.convertResourceType(item),
					}
					result <- r

					if item.Mtime.After(packLastMtime) {
						packLastMtime = item.Mtime
					}
				}
			}

			if packLastMtime.After(s.lastMtime) {
				s.lastMtime = packLastMtime
			}
		}

	}()

	return result
}
