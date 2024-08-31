package main

import (
	"bytes"
	"go.uber.org/zap"
	"yadisk-ds-sync/src/filesource"
)

func createLogger() *zap.SugaredLogger {
	unsugared, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return unsugared.Sugar()
}

func main() {
	log := createLogger()
	cfg, err := readConfig(log, "config.yaml")
	if err != nil {
		log.Fatal("read config failed", zap.Error(err))
	}

	b := []byte("Hello world")
	yd := filesource.NewYadisk(log, &cfg.Remote)
	err = yd.WriteFile("lala/dummy.txt", bytes.NewReader(b))
	if err != nil {
		log.Fatal("write file failed", zap.Error(err))
	}
}
