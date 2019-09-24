// Copyright 2019 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.package main

package models

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	log "github.com/inconshreveable/log15"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

var db *sql.DB
var dbVer = 2
var dbExpire = time.Duration(24) * time.Hour

// InitDB controls the opening connection to the database.
func InitDB(filepath string) error {
	var err error
	db, err = sql.Open("sqlite3", filepath)
	if err != nil {
		return err
	}
	err = db.Ping()
	return err
}

// CloseDB will close the database, only use when app closes.
func CloseDB() error {
	err := db.Close()
	return err
}

// LoadDB operates on the DB to load and/or migrate the database.
func LoadDB() error {
	err := createBwTestTable()
	if err != nil {
		return err
	}
	err = createEchoTable()
	if err != nil {
		return err
	}
	err = createTracerouteTable()
	if err != nil {
		return err
	}
	err = createTrHopTable()
	if err != nil {
		return err
	}
	version, err := getUserVersion()
	if err != nil {
		return err
	}
	log.Info("Loading database version", "version", version)
	// add successive migrations here
	if version < 1 {
		err = addColumn("bwtests", "Path TEXT")
	}
	if version < 2 {
		err = addColumn("bwtests", "Log TEXT")
	}
	if err != nil {
		return err
	}

	//set updated version
	if version < dbVer {
		err := setUserVersion(dbVer)
		if err != nil {
			return err
		}
		log.Info("Migrated to database version", "version", dbVer)
	}
	return err
}

// MaintainDatabase is a goroutine that runs independanly to cleanup the
// database according to the defined schedule.
func MaintainDatabase() {
	for {
		before := time.Now().Add(-dbExpire)

		count, err := DeleteBwTestItemsBefore(strconv.FormatInt(before.UnixNano()/1e6, 10))
		CheckError(err)
		if count > 0 {
			log.Warn(fmt.Sprint("Deleting ", count, " bwtests db rows older than", dbExpire))
		}

		count, err = DeleteEchoItemsBefore(strconv.FormatInt(before.UnixNano()/1e6, 10))
		CheckError(err)
		if count > 0 {
			log.Warn(fmt.Sprint("Deleting ", count, " echo db rows older than", dbExpire))
		}

		count, err = DeleteTracerouteItemsBefore(strconv.FormatInt(before.UnixNano()/1e6, 10))
		CheckError(err)
		if count > 0 {
			log.Warn(fmt.Sprint("Deleting ", count, " traceroute db rows older than", dbExpire))
		}

		count, err = DeleteTrHopItemsBefore(strconv.FormatInt(before.UnixNano()/1e6, 10))
		CheckError(err)
		if count > 0 {
			log.Warn(fmt.Sprint("Deleting ", count, " trhops db rows older than", dbExpire))
		}
		time.Sleep(dbExpire)
	}
}

func addColumn(table string, column string) error {
	sqlAddCol := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s;`, table, column)
	log.Info(sqlAddCol)
	_, err := db.Exec(sqlAddCol)
	return err
}

func getUserVersion() (int, error) {
	sqlGetVersion := `PRAGMA user_version;`
	rows, err := db.Query(sqlGetVersion)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	var version int
	for rows.Next() {
		err := rows.Scan(&version)
		if err != nil {
			return version, err
		}
	}
	err = rows.Err()
	if err != nil {
		return version, err
	}
	return version, nil
}

func setUserVersion(version int) error {
	sqlSetVersion := fmt.Sprintf(`PRAGMA user_version = %d;`, version)
	log.Info(sqlSetVersion)
	_, err := db.Exec(sqlSetVersion)
	return err
}
