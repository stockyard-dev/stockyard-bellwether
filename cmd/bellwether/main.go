package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/stockyard-dev/stockyard-bellwether/internal/server"
	"github.com/stockyard-dev/stockyard-bellwether/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9180"
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./bellwether-data"
	}

	db, err := store.Open(dataDir)
	if err != nil {
		log.Fatalf("bellwether: open database: %v", err)
	}
	defer db.Close()

	srv := server.New(db, server.DefaultLimits())

	fmt.Printf("\n  Bellwether — Self-hosted uptime and health monitor\n")
	fmt.Printf("  Questions? hello@stockyard.dev\n")
	fmt.Printf("  ─────────────────────────────────\n")
	fmt.Printf("  Dashboard:  http://localhost:%s/ui\n", port)
	fmt.Printf("  API:        http://localhost:%s/api\n", port)
	fmt.Printf("  Data:       %s\n", dataDir)
	fmt.Printf("  ─────────────────────────────────\n\n")

	srv.StartChecker()
	log.Printf("bellwether: listening on :%s (checker active)", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatalf("bellwether: %v", err)
	}
}
