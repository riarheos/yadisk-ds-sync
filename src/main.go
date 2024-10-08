package main

import (
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"yadisk-ds-sync/src/filesource"
	"yadisk-ds-sync/src/taskqueue"
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

func applyDiff(lf, rf filesource.FileSource, lt, rt *filesource.TreeNode, workers int) error {
	diff, err := lt.Compare(rt)
	if err != nil {
		return err
	}

	tq := taskqueue.NewTaskQueue(workers, false)

	for _, de := range diff {
		if de.Type == filesource.DirNode {
			if err = rf.MkDir(de.Name); err != nil {
				return err
			}
		} else {
			fn := de.Name
			tq.Push(func() error {
				f, err := lf.ReadFile(fn)
				if err != nil {
					return err
				}
				err = rf.WriteFile(fn, f)
				f.Close()
				if err != nil {
					return err
				}
				return nil
			})
		}
	}

	return tq.Run()
}

func dumpDiff(log *zap.SugaredLogger, lt, rt *filesource.TreeNode) error {
	diff, err := lt.Compare(rt)
	if err != nil {
		return err
	}
	for _, de := range diff {
		if de.Type == filesource.DirNode {
			log.Infof(" dir:  %s", de.Name)
		} else {
			log.Infof("file:  %s", de.Name)
		}
	}
	return nil
}

func main() {
	debug := flag.BoolP("debug", "d", false, "enable debug logging mode")
	trees := flag.BoolP("trees", "t", false, "dump corresponding trees")
	noDo := flag.BoolP("no-do", "n", false, "only show what would be done")
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
	if *trees {
		if err = lt.DumpToFile(log, "local_tree.yaml"); err != nil {
			log.Fatalf("failed to dump tree: %v", err)
		}
	}

	rt, err := remote.Tree()
	if err != nil {
		log.Fatalf("remote tree failed: %v", err)
	}
	if *trees {
		if err = rt.DumpToFile(log, "remote_tree.yaml"); err != nil {
			log.Fatalf("failed to dump tree: %v", err)
		}
	}

	if *noDo {
		log.Infof("Dumping local to remote")
		if err = dumpDiff(log, lt, rt); err != nil {
			log.Fatalf("failed to dump diff: %v", err)
		}
		log.Infof("Dumping remote to local")
		if err = dumpDiff(log, rt, lt); err != nil {
			log.Fatalf("failed to dump diff: %v", err)
		}
		return
	}

	log.Debug("Applying local to remote")
	if err = applyDiff(local, remote, lt, rt, cfg.Remote.Workers); err != nil {
		log.Fatalf("failed to apply diff: %v", err)
	}

	log.Debug("Applying remote to local")
	if err = applyDiff(remote, local, rt, lt, cfg.Remote.Workers); err != nil {
		log.Fatalf("failed to apply diff: %v", err)
	}

	log.Info("Successfully applied all changes")
}
