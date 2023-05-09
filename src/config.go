package main

import (
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"os"
	"yadisk-ds-sync/src/sources"
)

type config struct {
	Token string                 `yaml:"token"`
	Sync  [][]sources.SyncSource `yaml:"sync"`
}

func readConfig(log *zap.SugaredLogger, file string) (*config, error) {
	log.Debugf("Reading config from %v", file)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var c config
	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	if len(c.Sync) == 0 {
		return nil, fmt.Errorf("need at least one sync config to run")
	}

	return &c, nil
}
