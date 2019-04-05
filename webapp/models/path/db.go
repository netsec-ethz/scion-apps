package pathdb

import (
	"database/sql"

	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

func InitDB(filepath string) *sql.DB {
	var err error
	db, err := sql.Open("sqlite3", filepath+"?mode=ro")
	if CheckError(err) {
		panic(err)
	}
	err = db.Ping()
	if CheckError(err) {
		panic(err)
	}
	return db
}

func CloseDB(db *sql.DB) {
	err := db.Close()
	if CheckError(err) {
		panic(err)
	}
}
