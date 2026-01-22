package integration

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KVasquesMoviaUTN/cex-dex-arbitrage-challenge/internal/adapters/binance"
	"github.com/KVasquesMoviaUTN/cex-dex-arbitrage-challenge/internal/adapters/ethereum"
	"github.com/KVasquesMoviaUTN/cex-dex-arbitrage-challenge/internal/core/domain"
	"github.com/KVasquesMoviaUTN/cex-dex-arbitrage-challenge/internal/core/services"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	testifyMock "github.com/stretchr/testify/mock"
)

type MockBlockchainListener struct {
	testifyMock.Mock
}

func (m *MockBlockchainListener) SubscribeNewHeads(ctx context.Context) (<-chan *domain.Block, <-chan error, error) {
	args := m.Called(ctx)
	return args.Get(0).(<-chan *domain.Block), args.Get(1).(<-chan error), args.Error(2)
}

type MockNotificationService struct {
	testifyMock.Mock
	events chan domain.ArbitrageEvent
}

func NewMockNotificationService() *MockNotificationService {
	return &MockNotificationService{
		events: make(chan domain.ArbitrageEvent, 10),
	}
}

func (m *MockNotificationService) Broadcast(event domain.ArbitrageEvent) {
	m.events <- event
}

func TestEndToEndArbitrageFlow(t *testing.T) {
	binanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/depth" {
			// Return a mock order book where ETH price is ~2000 USDC
			response := map[string]interface{}{
				"lastUpdateId": 12345,
				"bids": [][]string{
					{"2000.00", "10.0"},
				},
				"asks": [][]string{
					{"2001.00", "10.0"},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer binanceServer.Close()



	ethServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
			ID     int           `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var result interface{}

		switch req.Method {
		case "eth_chainId":
			result = "0x1"
		case "eth_gasPrice":
			result = "0x4a817c800"
		case "eth_call":
			result = "0x000000000000000000000000000000000000000000000000000000007a31c700" +
				"0000000000000000000000000000000000000000000000000000000000000000" +
				"0000000000000000000000000000000000000000000000000000000000000000" +
				"00000000000000000000000000000000000000000000000000000000000186a0"
		default:
			result = "0x0"
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ethServer.Close()



	cexAdapter := binance.NewAdapter(binanceServer.URL)
	dexAdapter, err := ethereum.NewAdapter(ethServer.URL)
	assert.NoError(t, err)

	mockListener := &MockBlockchainListener{}
	mockNotifier := NewMockNotificationService()



	blockChan := make(chan *domain.Block)
	errChan := make(chan error)
	mockListener.On("SubscribeNewHeads", testifyMock.Anything).Return((<-chan *domain.Block)(blockChan), (<-chan error)(errChan), nil)



	cfg := services.Config{
		Symbol:        "ETHUSDC",
		TokenInAddr:   "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
		TokenOutAddr:  "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		TokenInDec:    18,
		TokenOutDec:   6,
		PoolFee:       3000,
		TradeSizes:    []*big.Int{big.NewInt(1000000000000000000)},
		MinProfit:     decimal.NewFromFloat(1.0),
		MaxWorkers:    1,
		CacheDuration: time.Second,
	}

	manager := services.NewManager(cfg, cexAdapter, dexAdapter, mockListener, mockNotifier)



	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := manager.Start(ctx)
		assert.NoError(t, err)
	}()



	time.Sleep(100 * time.Millisecond)

	block := &domain.Block{
		Number:    big.NewInt(1000),
		Timestamp: time.Now(),
	}
	blockChan <- block

	blockChan <- block

	timeout := time.After(2 * time.Second)
	var heartbeatReceived, opportunityReceived bool

	for {
		select {
		case event := <-mockNotifier.events:
			if event.Type == "HEARTBEAT" {
				heartbeatReceived = true

			} else if event.Type == "OPPORTUNITY" {
				opportunityReceived = true
				assert.Equal(t, "ETHUSDC", event.Data.Symbol)
				assert.Equal(t, "CEX -> DEX", event.Data.Direction)
				assert.Greater(t, event.Data.SpreadPct, 0.0)
				assert.Greater(t, event.Data.EstimatedProfit, 0.0)
				
				goto Done
			}
		case <-timeout:
			t.Fatal("Timeout waiting for arbitrage event")
		}
	}

Done:
	assert.True(t, heartbeatReceived, "Should receive heartbeat")
	assert.True(t, opportunityReceived, "Should receive arbitrage opportunity")
}
