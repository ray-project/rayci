package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type machineConfigInput struct {
	Name          string `json:"name"`
	Configuration string `json:"configuration"`
}

type machineConfig struct {
	id            int
	name          string
	configuration string
}

// handleGetRequest processes GET request to /machine/configuration that query the machine config based on name
func handleGetRequest(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Parse the query parameter
	name := r.URL.Query().Get("name")

	var rows *sql.Rows
	var err error
	// If name is specified, query the machine config with that name
	if name != "" {
		rows, err = db.Query("SELECT id, name, configuration FROM machine_config WHERE name = ?", name)
	} else {
		rows, err = db.Query("SELECT id, name, configuration FROM machine_config")
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Iterate through the queried rows and print out
	var machineConfigs []*machineConfig
	for rows.Next() {
		var machineConfig machineConfig
		err := rows.Scan(&machineConfig.id, &machineConfig.name, &machineConfig.configuration)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		machineConfigs = append(machineConfigs, machineConfig)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, c := range machineConfigs {
		fmt.Fprintf(w, "ID: %d, Name: %s, Configuration: %s\n", c.id, c.name, c.configuration)
	}
}

// a function to handle POST request in MachineConfig format to add a machine configuration to the database
func handlePostRequestMachineConfig(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Make sure the content type is JSON
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Parse the JSON body
	config := &machineConfigInput{}
	if err := json.NewDecoder(r.Body).Decode(config); err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Insert into database
	_, err = db.Exec("INSERT INTO machine_config (name, configuration) VALUES (?, ?)",
		config.Name, config.Configuration)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	dbPath := flag.String("db", "", "Path to .db file")
	flag.Parse()
	if *dbPath == "" {
		log.Fatalf("Database path is required")
	}
	if _, err := os.Stat(*dbPath); err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("File %s does not exist", *dbPath)
		}
		log.Fatalf("Error checking database file %s: %v", *dbPath, err)
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Error connecting to database: %s", err)
	}
	defer db.Close()

	// handle both GET and POST request to /machine/configuration
	http.HandleFunc("/machine/configuration", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetRequest(db, w, r)
		case http.MethodPost:
			handlePostRequestMachineConfig(db, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	log.Printf("Starting server")
	if err := http.ListenAndServe(":1234", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
