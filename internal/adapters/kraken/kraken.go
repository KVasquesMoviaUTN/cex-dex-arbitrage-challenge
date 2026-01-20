package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/ports"
	"github.com/shopspring/decimal"
)

const (
	BaseURL = "https://api.kraken.com"
)

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

// Kraken API response structures
type krakenDepthResponse struct {
	Error  []string               `json:"error"`
	Result map[string]krakenDepth `json:"result"`
}

type krakenDepth struct {
	Asks [][]string `json:"asks"` // [price, volume, timestamp]
	Bids [][]string `json:"bids"`
}

func (a *Adapter) GetOrderBook(ctx context.Context, symbol string) (*domain.OrderBook, error) {
	krakenSymbol := convertToKrakenSymbol(symbol)
	
	url := fmt.Sprintf("%s/0/public/Depth?pair=%s&count=100", BaseURL, krakenSymbol)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var krakenResp krakenDepthResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(krakenResp.Error) > 0 {
		return nil, fmt.Errorf("kraken api error: %v", krakenResp.Error)
	}

	var depth krakenDepth
	for _, v := range krakenResp.Result {
		depth = v
		break
	}

	orderBook := &domain.OrderBook{
		Asks:      make([]domain.PriceLevel, 0, len(depth.Asks)),
		Bids:      make([]domain.PriceLevel, 0, len(depth.Bids)),
		Timestamp: time.Now(),
	}

	for _, ask := range depth.Asks {
		if len(ask) < 2 {
			continue
		}
		price, err := decimal.NewFromString(ask[0])
		if err != nil {
			continue
		}
		quantity, err := decimal.NewFromString(ask[1])
		if err != nil {
			continue
		}
		orderBook.Asks = append(orderBook.Asks, domain.PriceLevel{
			Price:    price,
			Quantity: quantity,
		})
	}

	for _, bid := range depth.Bids {
		if len(bid) < 2 {
			continue
		}
		price, err := decimal.NewFromString(bid[0])
		if err != nil {
			continue
		}
		quantity, err := decimal.NewFromString(bid[1])
		if err != nil {
			continue
		}
		orderBook.Bids = append(orderBook.Bids, domain.PriceLevel{
			Price:    price,
			Quantity: quantity,
		})
	}

	return orderBook, nil
}

// convertToKrakenSymbol converts standard symbols to Kraken format
// ETHUSDC -> XETHZUSD
func convertToKrakenSymbol(symbol string) string {
	switch symbol {
	case "ETHUSDC":
		return "XETHZUSD"
	case "ETHUSD":
		return "XETHZUSD"
	case "BTCUSDC":
		return "XXBTZUSD"
	case "BTCUSD":
		return "XXBTZUSD"
	default:
		return symbol
	}
}
