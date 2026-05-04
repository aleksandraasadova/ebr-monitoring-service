package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/wsserver"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("no .env file found")
	}
	db, err := sql.Open("postgres", os.Getenv("DB_URL"))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	wsServer := wsserver.NewServer(":8081", db)
	slog.Info("starting server...")
	if err := wsServer.Start(); err != nil {
		slog.Error("server error", "err", err)
	}
}
