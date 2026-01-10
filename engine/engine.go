package engine

import (
	"context"
	"home-ci-cd/config"
	"home-ci-cd/repository"

	"go.uber.org/zap"
)

type Engine struct {
	cfg               config.Config
	configOrganizer   *config.Organizer
	repositoryManager *repository.Manager
}

func NewEngine(configOrganizer *config.Organizer) *Engine {
	cfg, err := configOrganizer.Load()
	if err != nil {
		zap.L().Error(err.Error())
		return nil
	}

	manager := repository.NewManager(cfg.Git)

	eng := &Engine{
		configOrganizer:   configOrganizer,
		repositoryManager: manager,
		cfg:               cfg,
	}

	return eng
}

func (e *Engine) Reload() {
	cfg, err := e.configOrganizer.Load()
	if err != nil {
		zap.L().Error(err.Error())
		return
	}

	_ = cfg

	// TODO
}

func (e *Engine) Run(ctx context.Context) error {
	if err := e.repositoryManager.Load(ctx, e.cfg.Repositories); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// TODO

	return nil
}
