package database

import (
    "database/sql"
    "log"

    _ "github.com/lib/pq"
)

// Open returns a connected *sql.DB with sane pool settings.
// Caller is responsible for calling db.Close() on shutdown.
func Open(databaseURL string) *sql.DB {
    db, err := sql.Open("postgres", databaseURL)
    if err != nil {
        log.Fatalf("database: open failed: %v", err)
    }

    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)

    if err := db.Ping(); err != nil {
        log.Fatalf("database: ping failed: %v", err)
    }

    log.Println("database: connected")
    return db
}
