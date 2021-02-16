package backcast

import (
	"context"
	"database/sql"
	"time"
)

type History struct {
	feed   string
	from   int
	cursor int
	err    error
}

type Diff struct {
	id      int
	diff    string
	created time.Time
}

func (h *History) Next(ctx context.Context, db *sql.DB) (Diff, error) {
	const sql = `SELECT id, diff, created FROM history WHERE feed=? AND id=?`
	var d Diff
	if err := db.QueryRowContext(ctx, sql, h.feed, h.cursor).Scan(&d.id, &d.diff, &d.created); err != nil {
		return d, err
	}
	h.cursor = d.id
	return d, nil
}

func (h *History) Walk(ctx context.Context, cb func(Diff, error), db *sql.DB) error {
	const sql = `SELECT id, diff, created FROM history WHERE feed=? AND id >= ?`
	rows, err := db.QueryContext(ctx, sql, h.feed, h.from)
	if err != nil {
		h.err = err
		return err
	}

	for rows.Next() {
		var d Diff
		if err := rows.Scan(&d.id, &d.diff, &d.created); err != nil {
			cb(d, err)
			continue
		}
		h.cursor = d.id
		cb(d, nil)
	}

	if err := rows.Close(); err != nil {
		h.err = err
		return err
	}

	return nil
}

func (h History) CommitDiff(ctx context.Context, diff string, db *sql.DB) error {
	const sql = `INSERT INTO history (feed, diff, created) VALUES(?,?,NOW())`
	_, err := db.ExecContext(ctx, sql, h.feed, diff)
	if err != nil {
		return err
	}
	return nil
}

func (h *History) Err() error {
	return h.err
}
