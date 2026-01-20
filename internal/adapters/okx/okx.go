package okx

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
	BaseURL = "https://www.okx.com"
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

// OKX API response structures
type okxResponse struct {
	Code string    `json:"code"`
	Msg  string    `json:"msg"`
	Data []okxData `json:"data"`
}

type okxData struct {
	Asks [][]string `json:"asks"` // [price, quantity, deprecated, num_orders]
	Bids [][]string `json:"bids"`
	Ts   string     `json:"ts"`
}

func (a *Adapter) GetOrderBook(ctx context.Context, symbol string) (*domain.OrderBook, error) {
	okxSymbol := convertToOKXSymbol(symbol)
	
	url := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=100", BaseURL, okxSymbol)
	
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

	var okxResp okxResponse
	if err := json.Unmarshal(body, &okxResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if okxResp.Code != "0" {
		return nil, fmt.Errorf("okx api error: %s - %s", okxResp.Code, okxResp.Msg)
	}

	if len(okxResp.Data) == 0 {
		return nil, fmt.Errorf("no data in response")
	}

	data := okxResp.Data[0]

	orderBook := &domain.OrderBook{
		Asks:      make([]domain.PriceLevel, 0, len(data.Asks)),
		Bids:      make([]domain.PriceLevel, 0, len(data.Bids)),
		Timestamp: time.Now(),
	}

	for _, ask := range data.Asks {
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

	for _, bid := range data.Bids {
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

// convertToOKXSymbol converts standard symbols to OKX format
// ETHUSDC -> ETH-USDC
func convertToOKXSymbol(symbol string) string {
	switch symbol {
	case "ETHUSDC":
		return "ETH-USDC"
	case "ETHUSD":
		return "ETH-USD"
	case "BTCUSDC":
		return "BTC-USDC"
	case "BTCUSD":
		return "BTC-USD"
	default:
		if len(symbol) >= 6 {
			return symbol[:len(symbol)-4] + "-" + symbol[len(symbol)-4:]
		}
		return symbol
	}
}
