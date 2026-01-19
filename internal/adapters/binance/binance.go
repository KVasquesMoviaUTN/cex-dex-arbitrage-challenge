package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/ports"
	"github.com/shopspring/decimal"
)

const baseURL = "https://api.binance.com/api/v3"

type Adapter struct {
	client *http.Client
}

func NewAdapter() ports.ExchangeAdapter {
	return &Adapter{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type depthResponse struct {
	LastUpdateID int64             `json:"lastUpdateId"`
	Bids         [][]string        `json:"bids"`
	Asks         [][]string        `json:"asks"`
}

// GetOrderBook fetches the current order book for the given symbol.
// Symbol should be like "ETHUSDC".
func (a *Adapter) GetOrderBook(ctx context.Context, symbol string) (*domain.OrderBook, error) {
	url := fmt.Sprintf("%s/depth?symbol=%s&limit=100", baseURL, symbol)
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
