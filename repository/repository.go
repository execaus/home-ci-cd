package repository

import "context"

type Repository interface {
	WatchBranches(ctx context.Context)
}
