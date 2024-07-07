package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	err        error
	filePath   = "chinook.db"
	memoryPath = "file:chinook.db?mode=memory&cache=shared"
)

type Db struct {
	FileDB *sql.DB
	MemDB  *sql.DB
}

func (db *Db) Init() (*Db, error) {
	// Connect to the file-based database
	db.FileDB, err = sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, err
	}
	db.MemDB, err = sql.Open("sqlite3", memoryPath)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func DbInstance() (*Db, error) {
	db := &Db{}
	db, err := db.Init()
	return db, err
}

func tableExists(db *sql.DB, tableName string) (bool, error) {
	query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s';", tableName)
	row := db.QueryRow(query)
	var name string
	err := row.Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func InitializeMemoryDB(memDB *sql.DB) {
	createTableStmt := `
		CREATE TABLE IF NOT EXISTS activities (
			id INTEGER NOT NULL PRIMARY KEY,
			time DATETIME NOT NULL,
			server TEXT
		);
	`
	_, err := memDB.Exec(createTableStmt)
	if err != nil {
		fmt.Printf("Error creating table in memory DB: %v\n", err)
		return
	}
	fmt.Println("Table 'activities' created in memory DB")
}

func RestoreInMemoryDBToFile(fileDB *sql.DB, memDB *sql.DB, tblName string) {
	ctx := context.TODO()

	// Check if the table exists in the memory database
	exists, err := tableExists(memDB, tblName)
	if err != nil {
		fmt.Println("Error checking table existence:", err)
		return
	}
	if !exists {
		fmt.Printf("Table %s does not exist in the memory database.\n", tblName)
	}

	// Generate a unique alias for the attached database
	alias := "file_db_" + fmt.Sprint(time.Now().UnixNano())

	// Attach the file-based database
	attachStmt := fmt.Sprintf("ATTACH DATABASE '%s' AS %s;", filePath, alias)
	_, err = memDB.ExecContext(ctx, attachStmt)
	if err != nil {
		fmt.Println("Error attaching database:", err)
		return
	}
	fmt.Println("Database attached")
	// Drop the existing table in the file-based database
	dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s_cache;", tblName)
	_, err = memDB.ExecContext(ctx, dropStmt)
	if err != nil {
		fmt.Println("Error dropping table:", err)
		return
	}
	fmt.Println("Table dropped in file-based database")
	// Copy the table from the memory database to the file-based database
	copyStmt := fmt.Sprintf("CREATE TABLE %s_cache AS SELECT * FROM %s.%s;", tblName, alias, tblName)
	_, err = memDB.ExecContext(ctx, copyStmt)
	if err != nil {
		fmt.Println("Error creating table:", err)
		return
	}
	fmt.Println("Table created in file-based database")

	// Detach the file-based database
	detachStmt := fmt.Sprintf("DETACH DATABASE %s;", alias)
	_, err = memDB.ExecContext(ctx, detachStmt)
	if err != nil {
		fmt.Println("Error detaching database:", err)
		return
	}
	fmt.Println("Database detached")

	fmt.Println("Restore from file DB to memory completed")
}
