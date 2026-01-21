package ports

import (
	"context"
	"math/big"

	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/domain"
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
	// GetQuote fetches the estimated output amount for a given input amount.
	// tokenIn and tokenOut are the addresses of the tokens.
	// fee is the pool fee tier (e.g., 500, 3000, 10000).
	GetQuote(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, fee int64) (*domain.PriceQuote, error)

	// GetQuoteExactOutput fetches the estimated input amount for a given output amount.
	GetQuoteExactOutput(ctx context.Context, tokenIn, tokenOut string, amountOut *big.Int, fee int64) (*domain.PriceQuote, error)

	// GetGasPrice fetches the current gas price from the network.
	GetGasPrice(ctx context.Context) (*big.Int, error)

	// GetSlot0 fetches the slot0 data (sqrtPriceX96, tick) for a pool.
	GetSlot0(ctx context.Context, tokenIn, tokenOut string, fee int64) (*domain.Slot0, error)
}

// BlockchainListener defines the interface for listening to blockchain events.
type BlockchainListener interface {
	// SubscribeNewHeads subscribes to new block headers.
	// It returns a channel that emits blocks and an error channel.
	SubscribeNewHeads(ctx context.Context) (<-chan *domain.Block, <-chan error, error)
}

// NotificationService defines the interface for broadcasting events to clients.
type NotificationService interface {
	Broadcast(event domain.ArbitrageEvent)
}
