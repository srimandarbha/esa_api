package model

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var (
	err error
)

type Db struct {
	Conn     *sql.DB
	filePath string
}

func (db *Db) Instance(filePath string) (*sql.DB, error) {
	// Connect to the file-based database
	db.Conn, err = sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, err
	}

	return db.Conn, nil
}

func RestoreInMemoryDBFromFile(filePath string, tblName string) (*sql.DB, *sql.DB, error) {
	db := Db{}
	diskDB, err := db.Instance(filePath)
	if err != nil {
		return nil, nil, err
	}
	memDB, _ := db.Instance("file::memory:?cache=shared")
	// Backup the file-based database to the in-memory database
	diskConn, err := diskDB.Conn(context.TODO())
	if err != nil {
		memDB.Close()
		return nil, nil, err
	}
	defer diskConn.Close()

	memConn, err := memDB.Conn(context.TODO())
	if err != nil {
		memDB.Close()
		return nil, nil, err
	}
	defer memConn.Close()

	// Use SQLite's backup API to copy the file-based database to the in-memory database
	_, err = memConn.ExecContext(context.TODO(), "ATTACH DATABASE ? AS file_db", filePath)
	if err != nil {
		memDB.Close()
		return nil, nil, err
	}

	_, err = memConn.ExecContext(context.TODO(), "DROP TABLE  IF EXISTS ?; create TEMP table emp as select * from file_db.?; DETACH DATABASE file_db", tblName)
	if err != nil {
		memDB.Close()
		return nil, nil, err
	}

	return diskDB, memDB, nil
}
