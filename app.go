package backcast

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

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

	router := httprouter.New()
	router.GET("/feed/:id", a.feedHandler)
	router.POST("/feed", a.createFeedHandler)
	router.PATCH("/feed/:id", a.updateFeedHandler)
	router.GET("/feed/:id/history", a.feedHistoryHandler)
	router.GET("/feed/:id/rss", a.feedRSSHandler)
	router.GET("/feed/:id/rss/:sha", a.feedSHARSSHandler)

	go a.startScanner(ctx)

	log.Printf("listening on %s", a.config.Listen)
	log.Fatal(http.ListenAndServe(a.config.Listen, router))
}

func (a *App) startScanner(ctx context.Context) error {
	for {
		select {
		case f := <-a.refresh:
			log.Printf("updating feed %d (%s)", f.ID, f.URL)
			if err := a.updateFeed(ctx, f); err != nil {
				log.Printf("failed to update feed %d (%s): %v", f.ID, f.URL, err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (a *App) updateFeed(ctx context.Context, f model.Feed) error {
	req, err := http.NewRequestWithContext(ctx, "GET", f.URL, nil)
	if f.Etag != "" {
		req.Header.Add("If-None-Match", f.Etag)
	}

	resp, err := http.Get(f.URL)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}

	if err = f.CommitDiff(ctx, string(body), tx); err != nil {
		tx.Rollback()
		return err
	}

	etag := resp.Header.Get("Etag")
	if etag != "" {
		if err = f.UpdateEtag(ctx, etag, tx); err != nil {
			tx.Rollback()
			return err
		}
	}

	tx.Commit()

	return nil
}

func (a *App) updateFeedHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	tx, err := a.db.Begin()
	if err != nil {
		jsonError(err, w)
		return
	}

	defer tx.Rollback()

	feed, err := model.GetFeed(ctx, ps.ByName("id"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	a.refresh <- feed
	fmt.Fprint(w, `{"status":"ok"}`)
}

func (a *App) createFeedHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	var f model.Feed

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&f); err != nil {
		jsonError(err, w)
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		jsonError(err, w)
		return
	}

	feed, err := model.CreateFeed(ctx, f.URL, tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	tx.Commit()

	a.refresh <- feed

	enc := json.NewEncoder(w)
	if err := enc.Encode(feed); err != nil {
		jsonError(err, w)
		return
	}
}

func (a *App) feedHistoryHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	tx, err := a.db.Begin()
	if err != nil {
		jsonError(err, w)
	}

	defer tx.Rollback()

	feed, err := model.GetFeed(ctx, ps.ByName("id"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	history, err := feed.History(ctx, tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(history); err != nil {
		jsonError(err, w)
		return
	}
}

func (a *App) feedRSSHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	tx, err := a.db.Begin()
	if err != nil {
		jsonError(err, w)
	}

	defer tx.Rollback()

	feed, err := model.GetFeed(ctx, ps.ByName("id"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	rss, err := feed.BuildFeed(ctx, "", tx)
	if err != nil {
		jsonError(err, w)
	}

	fmt.Fprint(w, rss)
}

func (a *App) feedSHARSSHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	tx, err := a.db.Begin()
	if err != nil {
		jsonError(err, w)
	}

	defer tx.Rollback()

	feed, err := model.GetFeed(ctx, ps.ByName("id"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	rss, err := feed.BuildFeed(ctx, ps.ByName("sha"), tx)
	if err != nil {
		jsonError(err, w)
	}

	fmt.Fprint(w, rss)
}

func (a *App) feedHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	tx, err := a.db.Begin()
	if err != nil {
		jsonError(err, w)
	}

	defer tx.Rollback()

	feed, err := model.GetFeed(ctx, ps.ByName("id"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(feed); err != nil {
		jsonError(err, w)
		return
	}
}

func jsonError(err error, w http.ResponseWriter) {
	fmt.Fprint(w, err)
}
