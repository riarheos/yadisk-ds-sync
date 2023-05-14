package main

import (
	"go.uber.org/zap"
	"yadisk-ds-sync/src/synca"
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
		log.Fatal(err)
	}

	for _, sync := range cfg.Sync {
		s, err := synca.New(log, sync, cfg.Token)
		if err != nil {
			log.Fatal(err)
		}
		err = s.Run()
		if err != nil {
			log.Fatal(err)
		}
		err = s.Destroy()
		if err != nil {
			log.Fatal(err)
		}
	}
}
