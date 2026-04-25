package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
	httpTransport "github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/http"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB // глобально, чтобы handler видел

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("no .env file found")
	}

	db, err = sql.Open("postgres", os.Getenv("DB_URL"))
	if err != nil {
		panic(err)
	}
	//defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// Compute absolute path to the web directory (assumes project root is working dir)
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	webDir := filepath.Join(wd, "web")

	// Serve login page at root and serve other static files from / (e.g. js/)
	fs := http.FileServer(http.Dir(webDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(webDir, "login.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	userRepo := repository.NewUserRepo(db)
	userService := service.NewUserService(userRepo)
	createUserHandler := httpTransport.CreateUserHandler(userService)
	http.HandleFunc("POST /api/v1/users", createUserHandler)

	authService := service.NewAuthService(userRepo)
	authLoginHandler := httpTransport.LoginHandler(authService)
	http.HandleFunc("POST /api/v1/auth/login", authLoginHandler)

	fmt.Println("server started on :8081")
	err = http.ListenAndServe(":8081", nil)
	if err != nil {
		fmt.Println("server error:", err)
	}
}
