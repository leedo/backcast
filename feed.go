package backcast

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/leedo/backcast/model"
)

type jsonErr struct {
	Error string `json:"error"`
}

func (a *App) updateFeedHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	tx, err := a.db.Begin()
	if err != nil {
		jsonInternalError(err, w)
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
		jsonInternalError(err, w)
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
		jsonInternalError(err, w)
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
		jsonInternalError(err, w)
	}

	defer tx.Rollback()

	f, err := model.GetFeed(ctx, ps.ByName("id"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	rv, err := f.GetCurrentRevision(ctx, tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	w.Header().Add("Content-Type", rv.ContentType)
	w.Header().Add("Content-Length", rv.ContentLength)

	if rv.Etag != "" {
		w.Header().Add("Etag", rv.Etag)
	} else {
		w.Header().Add("Etag", rv.Checksum)
	}

	rss, err := f.BuildFeed(ctx, "", tx)
	if err != nil {
		jsonError(err, w)
	}

	fmt.Fprint(w, rss)
}

func (a *App) feedRevisionRSSHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	tx, err := a.db.Begin()
	if err != nil {
		jsonInternalError(err, w)
	}

	defer tx.Rollback()

	f, err := model.GetFeed(ctx, ps.ByName("id"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	rv, err := f.GetRevision(ctx, ps.ByName("rev"), tx)
	if err != nil {
		jsonError(err, w)
		return
	}

	w.Header().Add("Content-Type", rv.ContentType)
	w.Header().Add("Content-Length", rv.ContentLength)

	if rv.Etag != "" {
		w.Header().Add("Etag", rv.Etag)
	} else {
		w.Header().Add("Etag", rv.Checksum)
	}

	rss, err := f.BuildFeed(ctx, ps.ByName("sha"), tx)
	if err != nil {
		jsonError(err, w)
	}

	fmt.Fprint(w, rss)
}

func (a *App) feedHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	tx, err := a.db.Begin()
	if err != nil {
		jsonInternalError(err, w)
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

func jsonInternalError(msg error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	enc := json.NewEncoder(w)
	out := jsonErr{msg.Error()}

	w.Header().Add("Content-Type", "application/javascript")

	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(w, `{"error": "unknown error"}`)
	}
}

func jsonError(msg error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	enc := json.NewEncoder(w)
	out := jsonErr{msg.Error()}

	w.Header().Add("Content-Type", "application/javascript")

	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(w, `{"error": "unknown error"}`)
	}
}
