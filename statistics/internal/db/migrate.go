package db

import (
	"embed"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func Migrate(options clickhouse.Options) error {
	// Create ClickHouse connection to migrate with
	db := clickhouse.OpenDB(&options)

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("clickhouse"); err != nil {
		return err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}

	return nil
}
