package domain_test

import (
	"testing"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCalculateEffectivePrice(t *testing.T) {
	// Setup OrderBook
	ob := &domain.OrderBook{
		Timestamp: time.Now(),
		Asks: []domain.PriceLevel{
			{Price: decimal.NewFromFloat(100.0), Amount: decimal.NewFromFloat(1.0)},
			{Price: decimal.NewFromFloat(101.0), Amount: decimal.NewFromFloat(2.0)},
			{Price: decimal.NewFromFloat(105.0), Amount: decimal.NewFromFloat(5.0)},
		},
		Bids: []domain.PriceLevel{
			{Price: decimal.NewFromFloat(99.0), Amount: decimal.NewFromFloat(1.0)},
			{Price: decimal.NewFromFloat(98.0), Amount: decimal.NewFromFloat(2.0)},
		},
	}

	tests := []struct {
		name          string
		side          string
		amount        decimal.Decimal
		expectedPrice decimal.Decimal
		expectedOk    bool
	}{
		{
			name:          "Buy 0.5 ETH (Full fill at best price)",
			side:          "buy",
			amount:        decimal.NewFromFloat(0.5),
			expectedPrice: decimal.NewFromFloat(100.0),
			expectedOk:    true,
		},
		{
			name:          "Buy 1.5 ETH (Partial fill 1st level, partial 2nd)",
			side:          "buy",
			amount:        decimal.NewFromFloat(1.5),
			// Cost: 1.0 * 100 + 0.5 * 101 = 100 + 50.5 = 150.5
			// Price: 150.5 / 1.5 = 100.3333...
			expectedPrice: decimal.NewFromFloat(150.5).Div(decimal.NewFromFloat(1.5)),
			expectedOk:    true,
		},
		{
			name:          "Buy 100 ETH (Not enough liquidity)",
			side:          "buy",
			amount:        decimal.NewFromFloat(100.0),
			expectedPrice: decimal.Zero,
			expectedOk:    false,
		},
		{
			name:          "Sell 1.0 ETH (Full fill at best bid)",
			side:          "sell",
			amount:        decimal.NewFromFloat(1.0),
			expectedPrice: decimal.NewFromFloat(99.0),
			expectedOk:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, ok := ob.CalculateEffectivePrice(tt.side, tt.amount)
			assert.Equal(t, tt.expectedOk, ok)
			if tt.expectedOk {
				assert.True(t, price.Equal(tt.expectedPrice), "Expected %s, got %s", tt.expectedPrice, price)
			}
		})
	}
}
