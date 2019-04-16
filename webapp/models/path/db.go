package pathdb

import (
	"database/sql"
)

func InitDB(filepath string) (*sql.DB, error) {
	//var err error
	db, err := sql.Open("sqlite3", filepath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CloseDB(db *sql.DB) error {
	err := db.Close()
	return err
}
