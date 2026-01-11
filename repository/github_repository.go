package repository

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"home-ci-cd/config"
	"home-ci-cd/db"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/client"
	"github.com/google/go-github/v81/github"
	"go.uber.org/zap"
)

const (
	BranchListPerPageOption    = 100
	watchPipelineSleepDuration = time.Second * 5
	GitObjectBlob              = "blob"
)

type GithubRepository struct {
	db              db.DB
	bufferDirectory string
	cfg             config.Repository
	client          *github.Client
}

func NewGithubRepository(client *github.Client, cfg config.Repository, bufferDirectory string, db db.DB) *GithubRepository {
	rand.Seed(time.Now().UnixNano())
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
					r.pipeline(ctx, branch, pipeline)
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

func (r *GithubRepository) pipeline(ctx context.Context, branch *github.Branch, pipeline config.BranchPipeline) {
	actualCommit := branch.Commit.GetSHA()
	branchName := branch.GetName()
	repoPath := filepath.Join(r.bufferDirectory, r.cfg.Repo+"_"+branchName)

	isNewVersion, err := r.isRepoNewVersion(ctx, actualCommit, branchName)
	if err != nil {
		zap.L().Error(err.Error())
		return
	}
	if !isNewVersion {
		zap.L().Info(fmt.Sprintf("Branch '%s' is up-to-date, skipping pipeline", branch.GetName()))
		return
	}

	if err = r.pullRepos(ctx, branchName, repoPath, actualCommit); err != nil {
		zap.L().Error(err.Error())
		return
	}
	defer r.clearDirectory(repoPath)

	imageReader, err := r.createImage(ctx, pipeline, repoPath)
	if err != nil {
		zap.L().Error(err.Error())
		return
	}
	defer func() {
		if err = imageReader.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if err = r.db.SaveLastCommit(ctx, r.cfg.Owner, r.cfg.Repo, branch.GetName(), actualCommit); err != nil {
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

func (r *GithubRepository) isRepoNewVersion(ctx context.Context, commit, branchName string) (bool, error) {
	lastCommit, err := r.db.GetLastCommit(ctx, r.cfg.Owner, r.cfg.Repo, branchName)
	if err != nil {
		zap.L().Error(err.Error())
		return false, err
	}

	return lastCommit == commit, nil
}

func (r *GithubRepository) pullRepos(ctx context.Context, branchName, repoPath, commit string) error {
	tree, _, err := r.client.Git.GetTree(ctx, r.cfg.Owner, r.cfg.Repo, commit, true)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	zap.L().Info(fmt.Sprintf(
		"downloaded %d files from branch '%s' at commit '%s' into buffer directory '%s/%s'",
		len(tree.Entries),
		branchName,
		commit,
		r.bufferDirectory,
		r.cfg.Repo,
	))

	errCh := make(chan error, 1)
	wg := &sync.WaitGroup{}
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, entry := range tree.Entries {
		if entry.GetType() != GitObjectBlob {
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
		return err
	}

	return nil
}

func (r *GithubRepository) createImage(ctx context.Context, pipeline config.BranchPipeline, repoPath string) (io.ReadCloser, error) {
	dockerfileName := getRandomString()
	imageTag := getRandomString()

	dockerfileDst := filepath.Join(repoPath, dockerfileName)

	dockerfileContent, err := os.ReadFile(pipeline.DockerFilePath)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	if err = os.WriteFile(dockerfileDst, dockerfileContent, 0644); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	dockerCli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	defer func() {
		if err = dockerCli.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	buildContext, err := r.getImageBuildContext(ctx, repoPath)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	imageBuildResp, err := dockerCli.ImageBuild(
		ctx,
		buildContext,
		build.ImageBuildOptions{
			Dockerfile: filepath.Base(dockerfileDst),
			Tags:       []string{imageTag},
			Remove:     true,
		},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return imageBuildResp.Body, nil
}

func (r *GithubRepository) getImageBuildContext(ctx context.Context, repoPath string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if err = file.Close(); err != nil {
				zap.L().Error(err.Error())
			}
		}()

		hdr := &tar.Header{
			Name:    relPath,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}

		if err = tw.WriteHeader(hdr); err != nil {
			return err
		}

		if _, err = io.Copy(tw, file); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func (r *GithubRepository) clearDirectory(path string) {
	if err := os.RemoveAll(path); err != nil {
		zap.L().Error(fmt.Sprintf("Failed to remove buffer repository directory '%s': %v", path, err))
	}
}

func getRandomString() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)

	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return hex.EncodeToString(b)
}
