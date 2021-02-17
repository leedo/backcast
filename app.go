package backcast

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/leedo/backcast/model"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	config  Config
	db      *sql.DB
	refresh chan model.Feed
}

func NewApp(c Config) (App, error) {
	a := App{config: c}

	if _, err := os.Stat(c.File); os.IsNotExist(err) {
		log.Printf("creating new database file %s", c.File)
		file, err := os.Create(c.File)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	db, err := sql.Open("sqlite3", a.dsn())
	if err != nil {
		return a, err
	}

	a.db = db
	a.refresh = make(chan model.Feed)

	return a, nil
}

func (a *App) dsn() string {
	return fmt.Sprintf("file:%s", a.config.File)
}

func (a *App) Run(ctx context.Context) {
	log.Println("initializing schema")
	if err := a.initSchema(ctx); err != nil {
		log.Fatal(err)
	}

	defer a.db.Close()

	go a.startScanner(ctx)

	router := httprouter.New()
	router.GET("/api/feed/:id", a.feedHandler)
	router.POST("/api/feed", a.createFeedHandler)
	router.PATCH("/api/feed/:id", a.updateFeedHandler)
	router.GET("/api/feed/:id/history", a.feedHistoryHandler)
	router.GET("/api/feed/:id/rss", a.feedRSSHandler)
	router.GET("/api/feed/:id/rss/:sha", a.feedSHARSSHandler)

	log.Printf("listening on %s", a.config.Listen)
	log.Fatal(http.ListenAndServe(a.config.Listen, router))
}

func (a *App) startScanner(ctx context.Context) error {
	t := time.NewTicker(1 * time.Minute)

	for {
		select {
		case f := <-a.refresh:
			log.Printf("updating feed %d (%s)", f.ID, f.URL)
			ok, err := a.updateFeed(ctx, f)
			if err != nil {
				log.Printf("failed to update feed %d (%s): %v", f.ID, f.URL, err)
			}
			if ok {
				log.Printf("updated feed %d (%s)", f.ID, f.URL)
			}
		case <-t.C:
			log.Println("scanning for stale feeds")
			if err := a.updateStaleFeeds(ctx); err != nil {
				log.Printf("%v", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (a *App) updateStaleFeeds(ctx context.Context) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}

	feeds, err := model.FindStaleFeeds(ctx, 1*time.Hour, 5, tx)
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, f := range feeds {
		log.Printf("checking feed %d (%s) for updates", f.ID, f.URL)
		ok, err := a.updateFeed(ctx, f)
		if err != nil {
			log.Printf("failed to update feed %d (%s): %v", f.ID, f.URL, err)
		}
		if ok {
			log.Printf("updated feed %d (%s)", f.ID, f.URL)
		}
	}

	tx.Commit()
	return nil
}

func (a *App) updateFeed(ctx context.Context, f model.Feed) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", f.URL, nil)
	if f.Etag != "" {
		req.Header.Add("If-None-Match", f.Etag)
	}

	resp, err := http.Get(f.URL)
	if err != nil {
		return false, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	tx, err := a.db.Begin()
	if err != nil {
		return false, err
	}

	ok, err := f.CommitDiff(ctx, string(body), tx)
	if err != nil {
		tx.Rollback()
		return false, err
	}

	etag := resp.Header.Get("Etag")
	if etag != "" {
		if err = f.UpdateEtag(ctx, etag, tx); err != nil {
			tx.Rollback()
			return false, err
		}
	}

	tx.Commit()

	return ok, nil
}
