package backcast

import (
	"database/sql"
	"fmt"
)

type App struct {
	config Config
	db     *sql.DB
}

func NewApp(c Config) (App, error) {
	app := App{c}
	db, err = sql.Open("sqlite", app.dsn())
	if err != nil {
		return app, err
	}
	app.db = db
	return app, nil
}

func (a *App) dsn() {
	return fmt.Sprintf("file:%s?cache=shared&mode=memory", a.config.file)
}
