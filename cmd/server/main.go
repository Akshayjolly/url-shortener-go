// Command server starts the URL shortener web server.
//
// Configuration (environment variables):
//
//	PORT      port to listen on (default 8080)
//	BASE_URL  public address used in short links (default http://localhost:PORT)
//	DB_DSN    MySQL DSN (default shortener:shortener@tcp(localhost:3306)/shortener?parseTime=true)
//	STORE     set to "memory" to run without MySQL (data is lost on restart)
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers the "mysql" driver with database/sql

	"github.com/Akshayjolly/url-shortener-go/internal/shortener"
	"github.com/Akshayjolly/url-shortener-go/internal/web"
)

func main() {
	port := getenv("PORT", "8080")
	baseURL := getenv("BASE_URL", "http://localhost:"+port)

	var store shortener.Store
	if os.Getenv("STORE") == "memory" {
		log.Println("using in-memory store")
		store = shortener.NewMemoryStore()
	} else {
		dsn := getenv("DB_DSN", "shortener:shortener@tcp(localhost:3306)/shortener?parseTime=true")
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
		defer db.Close()
		db.SetMaxOpenConns(10)
		db.SetConnMaxLifetime(5 * time.Minute)
		if err := db.Ping(); err != nil {
			log.Fatalf("cannot reach MySQL (is `docker compose up -d` running?): %v", err)
		}
		store = shortener.NewMySQLStore(db)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           web.NewServer(shortener.NewService(store), baseURL).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on %s", baseURL)
	log.Fatal(srv.ListenAndServe())
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
