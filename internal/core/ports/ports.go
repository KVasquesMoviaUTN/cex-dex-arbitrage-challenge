package ports

import (
	"context"
	"math/big"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
)

// ExchangeAdapter defines the interface for interacting with a CEX.
type ExchangeAdapter interface {
	// GetOrderBook fetches the current order book for the given symbol.
	GetOrderBook(ctx context.Context, symbol string) (*domain.OrderBook, error)
}

// PriceProvider defines the interface for interacting with a DEX.
type PriceProvider interface {
	// GetQuote fetches the estimated output amount for a given input amount.
	// tokenIn and tokenOut are the addresses of the tokens.
	// fee is the pool fee tier (e.g., 500, 3000, 10000).
	GetQuote(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, fee int64) (*domain.PriceQuote, error)
}

// BlockchainListener defines the interface for listening to blockchain events.
type BlockchainListener interface {
	// SubscribeNewHeads subscribes to new block headers.
	// It returns a channel that emits block numbers (or headers) and an error channel.
	SubscribeNewHeads(ctx context.Context) (<-chan *big.Int, <-chan error, error)
}
