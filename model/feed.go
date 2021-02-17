package model

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type Feed struct {
	ID         int64      `json:"id"`
	URL        string     `json:"url"`
	LastUpdate *time.Time `json:"last_update"`
	CreatedAt  time.Time  `json:"created_at"`
	Etag       string     `json:"etag"`
}

type Diff struct {
	ID        int64     `json:"id"`
	Diff      string    `json:"diff,omitempty"`
	Checksum  string    `json:"checksum"`
	CreatedAt time.Time `json:"created_at"`
}

func GetFeed(ctx context.Context, id string, db *sql.Tx) (Feed, error) {
	const query = `SELECT id, url, last_update, created_at, etag FROM feed WHERE id=?`
	var f Feed

	if err := db.QueryRowContext(ctx, query, id).Scan(&f.ID, &f.URL, &f.LastUpdate, &f.CreatedAt, &f.Etag); err != nil {
		return f, err
	}

	return f, nil
}

func CreateFeed(ctx context.Context, url string, db *sql.Tx) (Feed, error) {
	var f Feed
	now := time.Now()

	const query = `INSERT INTO feed (url, created_at) VALUES(?,?)`

	res, err := db.ExecContext(ctx, query, url, now)
	if err != nil {
		return f, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return f, err
	}

	return Feed{
		ID:        id,
		URL:       url,
		CreatedAt: now,
	}, nil
}

func (f Feed) CommitDiff(ctx context.Context, body string, db *sql.Tx) error {
	current, err := f.BuildFeed(ctx, "", db)
	if err != nil {
		return err
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(current, body, false)
	patch := dmp.PatchMake(current, diffs)

	if len(patch) == 0 {
		return nil
	}

	sum := sha1.Sum([]byte(body))
	hex := fmt.Sprintf("%x", sum)

	const query = `INSERT INTO history (feed, diff, checksum, created_at) VALUES(?,?,?,?)`
	_, err = db.ExecContext(ctx, query, f.ID, dmp.PatchToText(patch), hex, time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (f Feed) BuildFeed(ctx context.Context, checksum string, db *sql.Tx) (string, error) {
	const query = `SELECT checksum, diff FROM history WHERE feed=?`

	rows, err := db.QueryContext(ctx, query, f.ID)
	if err != nil {
		return "", err
	}

	var (
		feed string
		sha  string
	)

	dmp := diffmatchpatch.New()

	for rows.Next() {
		var (
			diff    string
			success []bool
		)

		if err := rows.Scan(&sha, &diff); err != nil {
			return "", err
		}

		patches, err := dmp.PatchFromText(diff)
		if err != nil {
			return "", err
		}

		feed, success = dmp.PatchApply(patches, feed)
		for i, s := range success {
			if !s {
				return "", fmt.Errorf("failed to apply patch: %v", patches[i])
			}
		}

		if checksum != "" && sha == checksum {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	if checksum != "" && sha != checksum {
		return "", fmt.Errorf("unknown revision %s", checksum)
	}

	return feed, nil
}

func (f Feed) UpdateEtag(ctx context.Context, etag string, db *sql.Tx) error {
	const query = `UPDATE feed SET etag=? WHERE id=?`
	_, err := db.ExecContext(ctx, query, f.ID)
	if err != nil {
		return err
	}
	return nil
}

func (f Feed) History(ctx context.Context, db *sql.Tx) ([]Diff, error) {
	const query = `SELECT id, checksum, created_at FROM history WHERE feed=?`
	var (
		diffs []Diff
		err   error
	)

	rows, err := db.QueryContext(ctx, query, f.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var d Diff
		if err := rows.Scan(&d.ID, &d.Checksum, &d.CreatedAt); err != nil {
			return nil, err
		}
		diffs = append(diffs, d)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return diffs, nil
}
