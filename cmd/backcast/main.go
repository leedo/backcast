package main

import (
	"context"
	"flag"
	"log"

	"github.com/leedo/backcast"
)

func main() {
	var c backcast.Config

	flag.StringVar(&c.File, "db-file", "state.db", "path to an sqlite database file")
	flag.StringVar(&c.Listen, "listen", "127.0.0.1:8080", "HTTP server listen interface and port")

	app, err := backcast.NewApp(c)
	if err != nil {
		log.Fatal(err)
	}

	app.Run(context.Background())
}
