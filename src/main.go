package main

import (
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"yadisk-ds-sync/src/filesource"
)

func createLogger(debug bool) *zap.SugaredLogger {
	cfg := zap.NewDevelopmentConfig()
	if !debug {
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	unsugared, err := cfg.Build(zap.WithCaller(false))
	if err != nil {
		panic(err)
	}
	return unsugared.Sugar()
}

func applyDiff(lf, rf filesource.FileSource, lt, rt *filesource.TreeNode) error {
	diff, err := lt.Compare(rt)
	if err != nil {
		return err
	}

	for _, de := range diff {
		if de.Type == filesource.DirNode {
			if err = lf.MkDir(de.Name); err != nil {
				return err
			}
		} else {
			f, err := rf.ReadFile(de.Name)
			if err != nil {
				return err
			}
			err = lf.WriteFile(de.Name, f)
			f.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	debug := flag.BoolP("debug", "d", false, "enable debug mode")
	configFile := flag.StringP("config", "c", "config.yaml", "configuration file")
	flag.Parse()

	log := createLogger(*debug)
	cfg, err := readConfig(log, *configFile)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	local := filesource.NewLocal(log, &cfg.Local)
	remote := filesource.NewYadisk(log, &cfg.Remote)

	lt, err := local.Tree()
	if err != nil {
		log.Fatalf("local tree failed: %v", err)
	}

	rt, err := remote.Tree()
	if err != nil {
		log.Fatalf("remote tree failed: %v", err)
	}

	log.Debug("Applying remote to local")
	if err = applyDiff(local, remote, lt, rt); err != nil {
		log.Fatalf("failed to apply diff: %v", err)
	}

	log.Debug("Applying local to remote")
	if err = applyDiff(remote, local, rt, lt); err != nil {
		log.Fatalf("failed to apply diff: %v", err)
	}

	log.Info("Successfully applied all changes")
}
