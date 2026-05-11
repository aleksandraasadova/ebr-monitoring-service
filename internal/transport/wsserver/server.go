package wsserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type WSServer interface {
	Start() error
	Stop() error
}

type wsSrv struct {
	srv   *http.Server
	wsUpg *websocket.Upgrader
}

func NewServer(addr string, handler http.Handler) WSServer {
	return &wsSrv{
		srv: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		wsUpg: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		}, // conn, err := s.wsUpg.Upgrade(w, r, nil)
	}
}

func (s *wsSrv) Start() error {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("server error: %v\n", err)
		}
	}()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
	<-sigint

	return s.Stop()
}

func (s *wsSrv) Stop() error {
	fmt.Println("server shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
