package mocks

import (
	"context"
	"math/big"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

// MockExchangeAdapter is a mock implementation of ports.ExchangeAdapter
type MockExchangeAdapter struct {
	mock.Mock
}

func (m *MockExchangeAdapter) GetOrderBook(ctx context.Context, symbol string) (*domain.OrderBook, error) {
	args := m.Called(ctx, symbol)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OrderBook), args.Error(1)
}

// MockPriceProvider is a mock implementation of ports.PriceProvider
type MockPriceProvider struct {
	mock.Mock
}

func (m *MockPriceProvider) GetQuote(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, fee int64) (*domain.PriceQuote, error) {
	args := m.Called(ctx, tokenIn, tokenOut, amountIn, fee)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PriceQuote), args.Error(1)
}

func (m *MockPriceProvider) GetGasPrice(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockPriceProvider) GetSlot0(ctx context.Context, tokenIn, tokenOut string, fee int64) (*domain.Slot0, error) {
	args := m.Called(ctx, tokenIn, tokenOut, fee)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Slot0), args.Error(1)
}

// MockBlockchainListener is a mock implementation of ports.BlockchainListener
type MockBlockchainListener struct {
	mock.Mock
}

func (m *MockBlockchainListener) SubscribeNewHeads(ctx context.Context) (<-chan *domain.Block, <-chan error, error) {
	args := m.Called(ctx)
	
	var ch2 <-chan error
	if args.Get(1) != nil {
		ch2 = args.Get(1).(<-chan error)
	}

	return ch1, ch2, args.Error(2)
}

// MockNotificationService is a mock implementation of ports.NotificationService
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) Broadcast(event domain.ArbitrageEvent) {
	m.Called(event)
}
