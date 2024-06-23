package model

import (
	_ "database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func dbConn() {
	log.Println("Connected to db")
}
