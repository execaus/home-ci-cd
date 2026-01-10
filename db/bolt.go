package db

import (
	"context"
	"fmt"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const (
	dbFilename       = "hommy.db"
	keyCommitPattern = "%s.%s.%s"
	commitBucket     = "commits"
)

type BoltDB struct {
	db *bbolt.DB
}

func (b *BoltDB) SaveLastCommit(ctx context.Context, owner, repo, branch, commit string) error {
	key := commitKey(owner, repo, branch)
	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(commitBucket))
		if err != nil {
			return err
		}
		return bucket.Put(key, []byte(commit))
	})
}

func (b *BoltDB) GetLastCommit(ctx context.Context, owner, repo, branch string) (string, error) {
	var commitStr string

	key := commitKey(owner, repo, branch)
	err := b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(commitBucket))
		if bucket == nil {
			return nil
		}
		val := bucket.Get(key)
		if val != nil {
			commitStr = string(val)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return commitStr, nil
}

func NewBoltDB() *BoltDB {
	db, err := bbolt.Open(dbFilename, 0666, nil)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	return &BoltDB{
		db: db,
	}
}

func (b *BoltDB) Close() error {
	return b.db.Close()
}

func commitKey(owner, repo, branch string) []byte {
	return []byte(fmt.Sprintf(keyCommitPattern, owner, repo, branch))
}
