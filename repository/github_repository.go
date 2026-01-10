package repository

import (
	"context"
	"fmt"
	"home-ci-cd/config"
	"home-ci-cd/db"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-github/v81/github"
	"go.uber.org/zap"
)

const (
	BranchListPerPageOption    = 100
	watchPipelineSleepDuration = time.Second * 5
)

type GithubRepository struct {
	db              db.DB
	bufferDirectory string
	cfg             config.Repository
	client          *github.Client
}

func NewGithubRepository(client *github.Client, cfg config.Repository, bufferDirectory string, db db.DB) *GithubRepository {
	return &GithubRepository{
		client:          client,
		cfg:             cfg,
		bufferDirectory: bufferDirectory,
		db:              db,
	}
}

func (r *GithubRepository) WatchBranches(ctx context.Context) {
	for _, pipeline := range r.cfg.BranchPipelines {
		branches, err := r.branchesForTemplate(ctx, pipeline.Template)
		if err != nil {
			zap.L().Error(err.Error())
			return
		}

		for _, branch := range branches {
			go func() {
				for {
					r.pipeline(ctx, branch)
					time.Sleep(watchPipelineSleepDuration)
				}
			}()
		}
	}

}

func (r *GithubRepository) branchesForTemplate(ctx context.Context, template string) ([]*github.Branch, error) {
	var branches []*github.Branch

	isProtected := false
	opts := &github.BranchListOptions{
		Protected: &isProtected,
		ListOptions: github.ListOptions{
			PerPage: BranchListPerPageOption,
		},
	}

	for {
		brs, resp, err := r.client.Repositories.ListBranches(ctx, r.cfg.Owner, r.cfg.Repo, opts)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}

		for _, branch := range brs {
			match, err := filepath.Match(template, branch.GetName())
			if err != nil {
				zap.L().Error(err.Error())
				return nil, err
			}
			if !match {
				continue
			}

			branches = append(branches, branch)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return branches, nil
}

func (r *GithubRepository) pipeline(ctx context.Context, branch *github.Branch) {
	currentCommit := branch.Commit.String()
	repoPath := filepath.Join(r.bufferDirectory, r.cfg.Repo+"_"+branch.GetName())

	defer func() {
		if err := os.RemoveAll(repoPath); err != nil {
			zap.L().Error(fmt.Sprintf("Failed to remove buffer repository directory '%s': %v", repoPath, err))
			return
		}
	}()

	lastCommit, err := r.db.GetLastCommit(ctx, r.cfg.Owner, r.cfg.Repo, branch.GetName())
	if err != nil {
		zap.L().Error(err.Error())
		return
	}

	if lastCommit == currentCommit {
		zap.L().Info(fmt.Sprintf(
			"Branch '%s' is up-to-date with last processed commit '%s', skipping pipeline",
			branch.GetName(),
			lastCommit,
		))
		return
	}

	tree, _, err := r.client.Git.GetTree(ctx, r.cfg.Owner, r.cfg.Repo, branch.Commit.GetSHA(), true)
	if err != nil {
		zap.L().Error(err.Error())
		return
	}

	zap.L().Info(fmt.Sprintf(
		"downloaded %d files from branch '%s' at commit '%s' into buffer directory '%s/%s'",
		len(tree.Entries),
		branch.GetName(),
		branch.Commit.GetSHA(),
		r.bufferDirectory,
		r.cfg.Repo,
	))

	errCh := make(chan error, 1)
	wg := &sync.WaitGroup{}
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, entry := range tree.Entries {
		if entry.GetType() != "blob" {
			continue
		}

		wg.Add(1)
		go r.createFile(cancelCtx, entry, repoPath, errCh, cancel, wg)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	if err = <-errCh; err != nil {
		zap.L().Error(err.Error())
		return
	}

	if err = r.db.SaveLastCommit(ctx, r.cfg.Owner, r.cfg.Repo, branch.GetName(), currentCommit); err != nil {
		zap.L().Error(err.Error())
		return
	}

	zap.L().Info(fmt.Sprintf(
		"Pipeline completed for branch '%s' at commit '%s'",
		branch.GetName(),
		branch.Commit.GetSHA(),
	))
}

func (r *GithubRepository) createFile(ctx context.Context, entry *github.TreeEntry, repoPath string, errCh chan<- error, cancel context.CancelFunc, wg *sync.WaitGroup) {
	defer wg.Done()

	path := entry.GetPath()

	fileContent, _, _, err := r.client.Repositories.GetContents(ctx, r.cfg.Owner, r.cfg.Repo, path, nil)
	if err != nil {
		select {
		case errCh <- err:
			cancel()
		default:
		}
		return
	}

	decoded, err := fileContent.GetContent()
	if err != nil {
		select {
		case errCh <- err:
			cancel()
		default:
		}
		return
	}

	fullPath := filepath.Join(repoPath, path)
	if err = os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
		select {
		case errCh <- err:
			cancel()
		default:
		}
		return
	}

	zap.L().Info(fmt.Sprintf("Writing file '%s' to buffer directory", fullPath))

	if err = os.WriteFile(fullPath, []byte(decoded), 0644); err != nil {
		select {
		case errCh <- err:
			cancel()
		default:
		}
		return
	}
}
