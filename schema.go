package backcast

import "context"

const schema = `
CREATE TABLE IF NOT EXISTS feed (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    url VARCHAR(2048) NOT NULL,
    etag VARCHAR(255) DEFAULT "",
    last_update DATETIME,
    created_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_url ON feed(url);
CREATE INDEX IF NOT EXISTS idx_last_update ON feed(last_update);

CREATE TABLE IF NOT EXISTS history (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    feed INTEGER NOT NULL,
    diff TEXT NOT NULL,
    checksum VARCHAR(40) NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_feed ON history(feed, id);
`

func (a *App) initSchema(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, schema)
	if err != nil {
		return err
	}
	return nil
}
