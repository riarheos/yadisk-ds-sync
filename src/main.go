package main

import (
	"go.uber.org/zap"
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

	l := newLocal(log, &cfg.Local)
	localTree, err := l.Tree()
	if err != nil {
		log.Fatal("tree failed", zap.Error(err))
	}
	localTree.dump(log, "")

	yd := newYadisk(log, &cfg.Remote)
	remoteTree, err := yd.Tree()
	if err != nil {
		log.Fatal("tree failed", zap.Error(err))
	}
	remoteTree.dump(log, "")
}
