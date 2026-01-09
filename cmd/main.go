package main

import (
	"flag"
	"home-ci-cd/config"
	"home-ci-cd/engine"

	"go.uber.org/zap"

	"context"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	zap.ReplaceGlobals(zap.Must(zap.NewProduction()))
}

func main() {
	ctx := context.Background()

	var configPath string
	flag.StringVar(&configPath, "c", "", "path to config file")
	flag.Parse()

	configOrganizer, err := config.NewOrganizer(configPath)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	eng := engine.NewEngine(configOrganizer)

	if err = eng.Run(ctx); err != nil {
		zap.L().Fatal(err.Error())
	}

	configOrganizer.AddChangeListeners(eng.Reload)

	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	zap.L().Info("application started")

	<-shutdownCtx.Done()
	zap.L().Info("shutdown signal received")

	if err = configOrganizer.Close(); err != nil {
		zap.L().Error(err.Error())
	}
	if err = zap.L().Sync(); err != nil {
		zap.L().Error("failed to sync logger", zap.Error(err))
	}

	zap.L().Info("application stopped gracefully")
}
