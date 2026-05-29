package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=5432 user=vmorbit dbname=vmorbit sslmode=disable")
	if err != nil {
		fmt.Printf("Open ERROR: %v\n", err)
		return
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fmt.Printf("Ping ERROR: %v\n", err)
		return
	}
	fmt.Println("Connected successfully via lib/pq!")
}
