package main

import (
	"flag"
	"log"
)

func main() {
	var c Config

	flag.StringVar(&c.file, "db-file", "state.db", "path to an sqlite database file")

	app, err := NewApp(c)
	if err != nil {
		log.Fatal(err)
	}
}
