package domain

import (
	"math/big"
	"time"

	"github.com/shopspring/decimal"
)

// OrderBook represents a snapshot of the order book from a CEX.
type OrderBook struct {
	Bids      []PriceLevel
	Asks      []PriceLevel
	Timestamp time.Time
}

// CalculateEffectivePrice calculates the average price to fill the given amount.
// Returns the average price and true if the amount can be filled, or 0 and false if not enough liquidity.
func (ob *OrderBook) CalculateEffectivePrice(side string, amount decimal.Decimal) (decimal.Decimal, bool) {
	var levels []PriceLevel
	if side == "buy" {
		levels = ob.Asks // Buying consumes Asks
	} else {
		levels = ob.Bids // Selling consumes Bids
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
		return decimal.Zero, false // Not enough liquidity
	}

	return totalCost.Div(amount), true
}

// PriceLevel represents a single price level in the order book.
type PriceLevel struct {
	Price  decimal.Decimal
	Amount decimal.Decimal
}

// PriceQuote represents a price quote from a DEX.
type PriceQuote struct {
	Price     decimal.Decimal // Effective price (OutputAmount / InputAmount)
	GasEstimate *big.Int
	Timestamp time.Time
}

// ArbitrageOpportunity represents a detected arbitrage opportunity.
type ArbitrageOpportunity struct {
	BuyOn      string          // "CEX" or "DEX"
	SellOn     string          // "CEX" or "DEX"
	BuyPrice   decimal.Decimal
	SellPrice  decimal.Decimal
	Profit     decimal.Decimal
	Timestamp  time.Time
}
