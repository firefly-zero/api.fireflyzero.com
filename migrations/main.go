package main

import (
	"context"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	command := os.Args[1]
	db, err := goose.OpenDBWithDriver("postgres", os.Getenv("API_POSTGRES_URL"))
	if err != nil {
		log.Fatalf("goose: failed to open DB: %v\n", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalf("goose: failed to close DB: %v\n", err)
		}
	}()
	goose.SetSequential(true)
	err = goose.RunContext(
		context.Background(),
		command, db,
		"migrations",
		os.Args[2:]...,
	)
	if err != nil {
		log.Fatalf("goose %v: %v", command, err) //nolint:gocritic
	}
}
