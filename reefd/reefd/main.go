package main

import (
	"flag"
	"log"

	"github.com/ray-project/rayci/reefd"
)

func main() {
	// dbPath := flag.String("db", "", "Path to .db file")
	// flag.Parse()

	// if *dbPath == "" {
	// 	log.Fatal("Database path is required")
	// }

	// if _, err := os.Stat(*dbPath); err != nil {
	// 	if os.IsNotExist(err) {
	// 		log.Fatalf("File %s does not exist", *dbPath)
	// 	}
	// 	log.Fatalf("Error checking database file %s: %v", *dbPath, err)
	// }

	// db, err := sql.Open("sqlite3", *dbPath)
	// if err != nil {
	// 	log.Fatalf("Error connecting to database: %s", err)
	// }
	// defer db.Close()

	config := &reefd.Config{}
	addr := flag.String("addr", "localhost:8000", "address to listen on")
	flag.Parse()

	log.Println("serving at:", *addr)
	if err := reefd.Serve(*addr, config); err != nil {
		log.Fatal(err)
	}
}
