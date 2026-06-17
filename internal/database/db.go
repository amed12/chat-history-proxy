package database

import (
	"database/sql"
	"log"

	"github.com/hellogod/chat-history-service/internal/security"
	_ "github.com/lib/pq"
)

// Open returns a connected *sql.DB with sane pool settings.
// Caller is responsible for calling db.Close() on shutdown.
func Open(databaseURL string) *sql.DB {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("database: open failed for %s: %v", security.MaskDatabaseURL(databaseURL), err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		log.Fatalf("database: ping failed for %s: %v", security.MaskDatabaseURL(databaseURL), err)
	}

	log.Println("database: connected")
	return db
}
