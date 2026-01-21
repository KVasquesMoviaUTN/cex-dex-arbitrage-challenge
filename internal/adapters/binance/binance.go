package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/domain"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/ports"
	"github.com/shopspring/decimal"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

const baseURL = "https://api.binance.com/api/v3"

type Adapter struct {
	client  *http.Client
	cb      *gobreaker.CircuitBreaker
	limiter *rate.Limiter
	baseURL string
}

func NewAdapter(baseURL string) ports.ExchangeAdapter {
	settings := gobreaker.Settings{
		Name:        "Binance",
		MaxRequests: 1,
		Interval:    0,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
	}

	return &Adapter{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		cb:      gobreaker.NewCircuitBreaker(settings),
		limiter: rate.NewLimiter(rate.Limit(20), 5), // 20 req/s, burst 5
		baseURL: baseURL,
	}
}

type depthResponse struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}


func (a *Adapter) GetOrderBook(ctx context.Context, symbol string) (*domain.OrderBook, error) {
	if err := a.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	body, err := a.cb.Execute(func() (interface{}, error) {
		url := fmt.Sprintf("%s/depth?symbol=%s&limit=100", a.baseURL, symbol)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("binance api returned status: %d", resp.StatusCode)
		}

		var depth depthResponse
		if err := json.NewDecoder(resp.Body).Decode(&depth); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return depth, nil
	})

	if err != nil {
		return nil, err
	}

	depth := body.(depthResponse)

	ob := &domain.OrderBook{
		Timestamp: time.Now(),
		Bids:      make([]domain.PriceLevel, 0, len(depth.Bids)),
		Asks:      make([]domain.PriceLevel, 0, len(depth.Asks)),
	}

	for _, b := range depth.Bids {
		price, _ := decimal.NewFromString(b[0])
		amount, _ := decimal.NewFromString(b[1])
		ob.Bids = append(ob.Bids, domain.PriceLevel{Price: price, Amount: amount})
	}

	for _, a := range depth.Asks {
		price, _ := decimal.NewFromString(a[0])
		amount, _ := decimal.NewFromString(a[1])
		ob.Asks = append(ob.Asks, domain.PriceLevel{Price: price, Amount: amount})
	}

	return ob, nil
}
