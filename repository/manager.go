package repository

import (
	"context"
	"errors"
	"fmt"
	"home-ci-cd/config"
	"home-ci-cd/pkg"
	"net/http"
	"sync"

	"github.com/google/go-github/v81/github"
	"go.uber.org/zap"
)

type Manager struct {
	githubClient *github.Client
}

func (m *Manager) Load(ctx context.Context, repositories []config.Repository) error {
	errs := make([]error, len(repositories))
	wg := &sync.WaitGroup{}

	for i, repository := range repositories {
		wg.Add(1)
		go func(errs []error, index int) {
			defer wg.Done()

			errs[index] = m.isRepositoryAccessible(ctx, repository.Owner, repository.RepoName)
			if errs[index] == nil {
				zap.L().Info(fmt.Sprintf("successfully connect repository %s/%s", repository.Owner, repository.RepoName))
			}
		}(errs, i)
	}

	wg.Wait()

	ok := true
	var accumError error
	for _, err := range errs {
		if err != nil {
			ok = false
			accumError = errors.Join(accumError, err)
		}
	}

	if !ok {
		return accumError
	}

	return nil
}

func NewManager(cfg config.Git) *Manager {
	m := &Manager{
		githubClient: github.NewClient(nil).WithAuthToken(cfg.Github.Token),
	}

	return m
}

func (m *Manager) isRepositoryAccessible(ctx context.Context, owner, repo string) error {
	_, err := pkg.RequestWithRetry[*http.Response](ctx, func(tCtx context.Context) (*http.Response, error) {
		_, resp, err := m.githubClient.Repositories.Get(tCtx, owner, repo)
		return resp.Response, err
	}, func(retryNumber int) {
		zap.L().Warn(fmt.Sprintf("Retrying access to repository %s/%s, attempt %d", owner, repo, retryNumber))
	})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
