package domain

import (
	"math/big"
	"time"

	"github.com/shopspring/decimal"
)

type OrderBook struct {
	Bids      []PriceLevel
	Asks      []PriceLevel
	Timestamp time.Time
}

type Block struct {
	Number    *big.Int
	Timestamp time.Time
}

type Slot0 struct {
	SqrtPriceX96 *big.Int
	Tick         *big.Int
}

// CalculateEffectivePrice calculates the average price to fill the given amount.
// Returns the average price and true if the amount can be filled, or 0 and false if not enough liquidity.
func (ob *OrderBook) CalculateEffectivePrice(side string, amount decimal.Decimal) (decimal.Decimal, bool) {
	var levels []PriceLevel
	if side == "buy" {
		levels = ob.Asks
	} else {
		levels = ob.Bids
	}

	remaining := amount
	totalCost := decimal.Zero

	for _, level := range levels {
		fill := level.Amount
		if fill.GreaterThan(remaining) {
			fill = remaining
		}
		
		cost := fill.Mul(level.Price)
		totalCost = totalCost.Add(cost)
		remaining = remaining.Sub(fill)
		
		if remaining.IsZero() {
			break
		}
	}

	if remaining.GreaterThan(decimal.Zero) {
		return decimal.Zero, false
	}

	return totalCost.Div(amount), true
}

type PriceLevel struct {
	Price  decimal.Decimal
	Amount decimal.Decimal
}

type PriceQuote struct {
	Price     decimal.Decimal // Effective price (OutputAmount / InputAmount)
	GasEstimate *big.Int
	Timestamp time.Time
}

type ArbitrageOpportunity struct {
	BuyOn      string
	SellOn     string
	BuyPrice   decimal.Decimal
	SellPrice  decimal.Decimal
	Profit     decimal.Decimal
	Timestamp  time.Time
}

type TradeData struct {
	CexPrice        float64 `json:"cexPrice"`
	DexPrice        float64 `json:"dexPrice"`
	SpreadPct       float64 `json:"spreadPct"`
	EstimatedProfit float64 `json:"estimatedProfit"`
	GasCost         float64 `json:"gasCost"`
	Symbol          string  `json:"symbol"`
}

type ArbitrageEvent struct {
	Type        string     `json:"type"` // "HEARTBEAT" | "OPPORTUNITY"
	BlockNumber uint64     `json:"blockNumber"`
	Timestamp   time.Time  `json:"timestamp"`
	Data        *TradeData `json:"data,omitempty"`
}
