package models

import (
    "database/sql"
    "log"
)

var db *sql.DB

// InitDB controls the opening connection to the database.
func InitDB(filepath string) {
    var err error
    db, err = sql.Open("sqlite3", filepath)
    if err != nil {
        log.Panic(err)
    }

    if err = db.Ping(); err != nil {
        log.Panic(err)
    }
}

// CloseDB will close the database, only use when app closes.
func CloseDB() {
    var err error
    if err = db.Close(); err != nil {
        log.Panic(err)
    }
}
