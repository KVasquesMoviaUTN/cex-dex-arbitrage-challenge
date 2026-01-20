package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestServer_Broadcast(t *testing.T) {
	// Setup Server
	server := NewServer()
	
	// Create test server
	s := httptest.NewServer(http.HandlerFunc(server.handleConnections))
	defer s.Close()

	// Start message handler
	go server.handleMessages()

	// Convert http URL to ws URL
	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect a client
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	assert.NoError(t, err)
	defer ws.Close()

	// Wait for connection registration
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	event := domain.ArbitrageEvent{
		Type:        "TEST_EVENT",
		BlockNumber: 12345,
		Timestamp:   time.Now(),
	}
	server.Broadcast(event)

	// Read message from client
	var received domain.ArbitrageEvent
	err = ws.ReadJSON(&received)
	assert.NoError(t, err)

	assert.Equal(t, event.Type, received.Type)
	assert.Equal(t, event.BlockNumber, received.BlockNumber)
}
