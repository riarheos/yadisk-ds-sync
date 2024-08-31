package main

import (
	"errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"os"
	"time"
	"yadisk-ds-sync/src/filesource"
)

type config struct {
	Local  filesource.LocalConfig  `yaml:"local"`
	Remote filesource.YadiskConfig `yaml:"remote"`
}

func readConfig(log *zap.SugaredLogger, filename string) (*config, error) {
	var b []byte
	var err error

	log.Debugf("Reading configuration from %s", filename)
	if b, err = os.ReadFile(filename); err != nil {
		return nil, err
	}

	c := &config{}
	if err = yaml.Unmarshal(b, c); err != nil {
		return nil, err
	}

	if c.Local.Path == "" {
		return nil, errors.New("local path is required")
	}
	if c.Remote.Path == "" {
		return nil, errors.New("remote path is required")
	}
	if c.Remote.Token == "" {
		return nil, errors.New("remote token is required")
	}
	if c.Remote.Timeout == 0 {
		c.Remote.Timeout = 30 * time.Second
	}

	return c, nil
}
