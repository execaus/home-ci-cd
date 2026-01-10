package db

import (
	"context"
	"io"
)

type DB interface {
	io.Closer
	SaveLastCommit(ctx context.Context, owner, repo, branch, commit string) error
	GetLastCommit(ctx context.Context, owner, repo, branch string) (string, error)
}
