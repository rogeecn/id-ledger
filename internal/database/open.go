package database

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func Open(path string) (*sql.DB, error) {
	connection, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	connection.SetMaxOpenConns(1)
	if _, err := connection.Exec(schema); err != nil {
		connection.Close()
		return nil, err
	}
	return connection, nil
}
