package backcast

import (
	"context"
	"database/sql"
	"log"
	"time"
)

const createSchema = `
CREATE TABLE IF NOT EXISTS backcast_schema (
    version INT PRIMARY KEY NOT NULL,
    applied DATETIME NOT NULL
)
`

const schemaExists = `SELECT version FROM backcast_schema WHERE version=?`
const updateSchema = `INSERT INTO backcast_schema (version, applied) VALUES(?,?)`

var migrations []string

func init() {
	migrate(`
CREATE TABLE feed (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    url VARCHAR(2048) NOT NULL,
    etag VARCHAR(255) DEFAULT "",
    last_update DATETIME,
    created_at DATETIME NOT NULL
)`)

	migrate(`
CREATE UNIQUE INDEX idx_url ON feed(url);
CREATE INDEX idx_last_update ON feed(last_update);
`)

	migrate(`
CREATE TABLE history (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    feed INTEGER NOT NULL,
    diff TEXT NOT NULL,
    checksum VARCHAR(40) NOT NULL,
    created_at DATETIME NOT NULL
)`)

	migrate(`
CREATE UNIQUE INDEX idx_feed ON history(feed, id);
`)
}

func migrate(query string) {
	migrations = append(migrations, query)
}

func (a *App) initSchema(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, createSchema)
	if err != nil {
		return err
	}

	for i, m := range migrations {
		_, err := a.db.QueryContext(ctx, schemaExists, i)
		if err == sql.ErrNoRows {
			log.Println(m)
			if _, err := a.db.ExecContext(ctx, m); err != nil {
				return err
			}
			if _, err := a.db.ExecContext(ctx, updateSchema, i, time.Now()); err != nil {
				return err
			}
			log.Printf("upgraded schema to version %d", i)
		} else if err != nil {
			return err
		} else {
			log.Printf("skipping schema version %d", i)
		}
	}

	return nil
}
