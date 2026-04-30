package wsserver

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
	transport "github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/http"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
	_ "github.com/lib/pq"
)

type wsSrv struct {
	mux *http.ServeMux
	srv *http.Server
}

func NewServer(addr string, db *sql.DB) WSServer {
	m := http.NewServeMux()

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	webDir := filepath.Join(wd, "web")

	// Serve login page at root and serve other static files from / (e.g. js/)
	fs := http.FileServer(http.Dir(webDir))
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

	createUserHandler := transport.CreateUserHandler(userService)
	m.Handle("POST /api/v1/users",
		middleware.JWT(middleware.RequireRole("admin")(createUserHandler)))

	authService := service.NewAuthService(userRepo)

	authLoginHandler := transport.LoginHandler(authService)
	m.HandleFunc("POST /api/v1/auth/login", authLoginHandler)

	return &wsSrv{
		mux: m,
		srv: &http.Server{
			Addr:    addr,
			Handler: m,
		},
	}
}

type WSServer interface {
	Start() error
	Stop() error
}

func (s *wsSrv) Start() error {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed { // если не штатное завершение работы сервера
			fmt.Printf("server error: %v\n", err)
		}
	}()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM) // SIGINT - Ctrl+C, SIGTERM - kill pid
	<-sigint

	return s.Stop()
}

func (s *wsSrv) Stop() error {
	fmt.Println("server shutting down...")
	// специфика системы требует больше времени на таймаут, но в тестовом режиме 5 секунд
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
