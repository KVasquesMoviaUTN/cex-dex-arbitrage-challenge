package websocket

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/KVasquesMoviaUTN/cex-dex-arbitrage-challenge/internal/core/domain"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	clients   map[*websocket.Conn]bool
	broadcast chan domain.ArbitrageEvent
	mu        sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan domain.ArbitrageEvent),
	}
}

func (s *Server) Start(addr string) {
	http.HandleFunc("/ws", s.handleConnections)
	
	go s.handleMessages()

	slog.Info("WebSocket server starting", "addr", addr)
	
	// Wrap the default mux with CORS middleware
	handler := corsMiddleware(http.DefaultServeMux)
	
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("WebSocket server failed", "error", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WS upgrade failed", "error", err)
		return
	}
	defer ws.Close()

	s.mu.Lock()
	s.clients[ws] = true
	s.mu.Unlock()

	slog.Info("New WebSocket client connected")

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			s.mu.Lock()
			delete(s.clients, ws)
			s.mu.Unlock()
			break
		}
	}
}

func (s *Server) handleMessages() {
	for {
		msg := <-s.broadcast
		
		s.mu.RLock()
		for client := range s.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				slog.Error("WS write failed", "error", err)
				client.Close()

			}
		}
		s.mu.RUnlock()
	}
}

func (s *Server) Broadcast(event domain.ArbitrageEvent) {
	s.broadcast <- event
}
