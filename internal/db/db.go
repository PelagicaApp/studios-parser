package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Region struct {
	Type       string
	RegionCode string
	RegionName string
}

type Studio struct {
	ID       int64
	Name     string
	LogoPath *string
	Regions  []Region
}

func OpenDatabase(dbPath string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := ensureCompaniesSchema(database); err != nil {
		database.Close()
		return nil, err
	}

	return database, nil
}
