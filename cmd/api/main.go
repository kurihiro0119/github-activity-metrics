package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kurihiro0119/github-activity-metrics/internal/aggregator"
	"github.com/kurihiro0119/github-activity-metrics/internal/api"
	"github.com/kurihiro0119/github-activity-metrics/internal/config"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage/postgres"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage/sqlite"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize storage
	var store storage.Storage
	switch cfg.StorageType {
	case "postgres":
		store, err = postgres.NewPostgresStorage(cfg.PostgresURL)
		if err != nil {
			log.Fatalf("Failed to initialize PostgreSQL storage: %v", err)
		}
	default:
		store, err = sqlite.NewSQLiteStorage(cfg.SQLitePath)
		if err != nil {
			log.Fatalf("Failed to initialize SQLite storage: %v", err)
		}
	}
	defer store.Close()

	// Initialize aggregator
	agg := aggregator.NewAggregator(store)

	// Initialize handler
	handler := api.NewHandler(agg)

	// Setup routes
	router := api.SetupRoutes(handler)

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.APIHost, cfg.APIPort)
	fmt.Printf("Starting API server on %s\n", addr)
	fmt.Printf("Storage type: %s\n", cfg.StorageType)

	if err := router.Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
}
