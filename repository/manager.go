package repository

import (
	"context"
	"errors"
	"fmt"
	"home-ci-cd/config"
	"home-ci-cd/db"
	"home-ci-cd/pkg"
	"net/http"
	"sync"

	"github.com/google/go-github/v81/github"
	"go.uber.org/zap"
)

type Manager struct {
	githubClient    *github.Client
	repositories    []config.Repository
	bufferDirectory string
	db              db.DB
}

func NewManager(cfg config.Git, bufferDirectory string, database db.DB) *Manager {
	m := &Manager{
		githubClient:    github.NewClient(nil).WithAuthToken(cfg.Github.Token),
		bufferDirectory: bufferDirectory,
		db:              database,
	}

	return m
}

func (m *Manager) Load(ctx context.Context, repositories []config.Repository) error {
	errs := make([]error, len(repositories))
	wg := &sync.WaitGroup{}

	for i, repository := range repositories {
		wg.Add(1)
		go func(errs []error, index int) {
			defer wg.Done()

			errs[index] = m.isRepositoryAccessible(ctx, repository.Owner, repository.Repo)
			if errs[index] == nil {
				zap.L().Info(fmt.Sprintf("successfully connect repository %s/%s", repository.Owner, repository.Repo))
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

	m.repositories = repositories

	return nil
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

func (m *Manager) GetAll() ([]Repository, error) {
	r := make([]Repository, len(m.repositories))

	for i, repository := range m.repositories {
		switch repository.Type {
		case config.GithubType:
			r[i] = NewGithubRepository(m.githubClient, repository, m.bufferDirectory, m.db)
		default:
			zap.L().Error(ErrInvalidGitType.Error())
			return nil, ErrInvalidGitType
		}
	}

	return r, nil
}
