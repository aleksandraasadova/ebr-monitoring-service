package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "github.com/aleksandraasadova/ebr-monitoring-service/docs/swagger"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
	transport "github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/http"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/mqtt"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/wsserver"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// @title           EBR Monitoring Service API
// @version         1.0
// @description     API сервиса мониторинга процесса эмульсификации (партии, рецепты, пользователи).
// @host            localhost:8081
// @BasePath        /
// @schemes         http

// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     JWT токен в формате: Bearer <token>
func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("no .env file found")
	}

	db, err := connectDB(os.Getenv("DB_URL"))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	mqttClient := mqtt.NewClient(os.Getenv("MQTT_BROKER"), os.Getenv("CLIENT_ID"))
	if err := mqttClient.Connect(); err != nil {
		slog.Warn("MQTT connect failed", "err", err)
	} else {
		slog.Info("MQTT connected", "broker", os.Getenv("MQTT_BROKER"))
		defer mqttClient.Disconnect(250)
	}

	userRepo := repository.NewUserRepo(db)
	recipeRepo := repository.NewRecipeRepo(db)
	batchRepo := repository.NewBatchRepo(db)

	userService := service.NewUserService(userRepo)
	authService := service.NewAuthService(userRepo)
	recipeService := service.NewRecipeService(recipeRepo)
	batchService := service.NewBatchService(batchRepo, recipeRepo)

	mux := transport.NewRouter(transport.RouterDeps{
		WebDir:        filepath.Join(wd, "web"),
		UserService:   userService,
		AuthService:   authService,
		RecipeService: recipeService,
		BatchService:  batchService,
	})

	srv := wsserver.NewServer(":8081", mux)
	slog.Info("starting server...")
	if err := srv.Start(); err != nil {
		slog.Error("server error", "err", err)
	}
}

func connectDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return db, nil
}
