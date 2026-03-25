package main

import (
	"context"
	"diploma/usermanagement-service/internal/config"
	"diploma/usermanagement-service/internal/db"
	"log"
)

func main() {
	ctx := context.Background()

	cfg := config.Load()

	pool, err := db.NewPool(ctx, db.Config{
		DSN:         cfg.DB.DSN,
		MaxConns:    cfg.DB.MaxConns,
		MinConns:    cfg.DB.MinConns,
		PingTimeout: cfg.DB.PingTimeout,
	})
	if err != nil {
		log.Fatal("DB connection error:", err)
	}
	defer pool.Close()

	log.Println("DB connected successfully ✅")

	var x int
	err = pool.QueryRow(ctx, "SELECT 1").Scan(&x)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("DB test:", x)
}
