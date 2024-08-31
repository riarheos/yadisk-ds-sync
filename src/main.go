package main

import (
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

	l := filesource.NewLocal(log, &cfg.Local)
	localTree, err := l.Tree()
	if err != nil {
		log.Fatal("tree failed", zap.Error(err))
	}

	yd := filesource.NewYadisk(log, &cfg.Remote)
	remoteTree, err := yd.Tree()
	if err != nil {
		log.Fatal("tree failed", zap.Error(err))
	}

	diff, err := localTree.Compare(remoteTree)
	for _, x := range diff {
		log.Info("diff", zap.Any("x", x))
	}
}
