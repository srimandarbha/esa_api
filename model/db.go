package model

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func FlatDBInstance(filePath string) (*sql.DB, error) {
	// Connect to the file-based database
	diskDB, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, err
	}
	defer diskDB.Close()

	return diskDB, nil
}

func MemDBInstance() (*sql.DB, error) {
	// Connect to the file-based database
	// Create an in-memory database
	memDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	defer memDB.Close()
	return memDB, nil
}

func RestoreInMemoryDBFromFile(filePath string, tblName string) (*sql.DB, error) {
	diskDB, err := FlatDBInstance("chinook.db")
	if err != nil {
		return nil, err
	}
	memDB, _ := MemDBInstance()
	// Backup the file-based database to the in-memory database
	diskConn, err := diskDB.Conn(context.TODO())
	if err != nil {
		memDB.Close()
		return nil, err
	}
	defer diskConn.Close()

	memConn, err := memDB.Conn(context.TODO())
	if err != nil {
		memDB.Close()
		return nil, err
	}
	defer memConn.Close()

	// Use SQLite's backup API to copy the file-based database to the in-memory database
	_, err = memConn.ExecContext(context.TODO(), "ATTACH DATABASE ? AS file_db", filePath)
	if err != nil {
		memDB.Close()
		return nil, err
	}

	_, err = memConn.ExecContext(context.TODO(), "DROP TABLE  IF EXISTS ?; create TEMP table emp as select * from file_db.?; DETACH DATABASE file_db", tblName)
	if err != nil {
		memDB.Close()
		return nil, err
	}

	return memDB, nil
}
