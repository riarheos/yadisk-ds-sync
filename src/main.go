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
	log := createLogger()
	cfg, err := readConfig(log, "config.yaml")
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
