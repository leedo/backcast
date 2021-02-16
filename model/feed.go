package main

import (
	"context"
	"database/sql"
)

type Feed struct {
	id  string
	url string
}

func GetFeed(ctx context.Context, id string, db *sql.DB) (Feed, error) {
	const sql = `SELECT id, url FROM feed WHERE id=?`
	var f Feed
	if err := db.QueryRowContext(ctx, sql, id).Scan(&f.id, &f.url); err != nil {
		return f, err
	}
	return f, nil
}

func (f Feed) History(ctx context.Context, db *sql.DB) History {
	return History{
		feed:   f.id,
		cursor: 0,
	}
}
