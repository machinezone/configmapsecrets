package mzlog

import (
	"os"
	"runtime"

	"github.com/machinezone/configmapsecrets/pkg/version"
	"go.uber.org/zap"
)

// Process logs info about the process to the logger.
func Process(logger *zap.Logger) {
	logger.Info(
		"running",
		zap.String("command", os.Args[0]),
		zap.Strings("args", os.Args[1:]),
	)
	logger.Info(
		"version",
		zap.String("binary", version.Binary()),
		zap.String("repo", version.Repo()),
		zap.String("branch", version.Branch()),
		zap.String("version", version.Version()),
		zap.String("revision", version.Revision()),
		zap.Time("build_time", version.BuildTime()),
	)
	logger.Info(
		"runtime",
		zap.String("version", runtime.Version()),
		zap.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		zap.Int("cpus", runtime.NumCPU()),
	)
}
