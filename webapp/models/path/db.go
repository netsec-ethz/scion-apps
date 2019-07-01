package pathdb

import (
	"database/sql"
)

// InitDB controls the opening connection to the database.
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

// CloseDB will close the database, only use when app closes.
func CloseDB(db *sql.DB) error {
	err := db.Close()
	return err
}
