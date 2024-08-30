package main

import (
	"go.uber.org/zap"
	"os"
	"os/signal"
	"sync"
	"yadisk-ds-sync/src/sources"
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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	var wg sync.WaitGroup
	wg.Add(len(cfg.Sync))

	syncas := make(chan *synca.Synca, len(cfg.Sync))

	for _, syncSources := range cfg.Sync {
		go func(syncSources []sources.SyncSource) {
			s, err := synca.New(log, syncSources, cfg.Token)
			if err != nil {
				log.Fatal(err)
			}

			syncas <- s

			err = s.Run()
			if err != nil {
				log.Fatal(err)
			}

			err = s.Destroy()
			if err != nil {
				log.Fatal(err)
			}

			wg.Done()
		}(syncSources)
	}

	<-sig
	for i := 0; i < len(cfg.Sync); i++ {
		s := <-syncas
		s.Done <- true
	}

	wg.Wait()
}
