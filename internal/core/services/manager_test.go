package services_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/ports/mocks"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/services"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
)

func TestManager_ProcessBlock(t *testing.T) {
	// Setup
	mockCEX := new(mocks.MockExchangeAdapter)
	mockDEX := new(mocks.MockPriceProvider)
	mockListener := new(mocks.MockBlockchainListener)
	mockNotifier := new(mocks.MockNotificationService)

	cfg := services.Config{
		Symbol:        "ETHUSDC",
		TokenInAddr:   "0xWETH",
		TokenOutAddr:  "0xUSDC",
		TokenInDec:    18,
		TokenOutDec:   6,
		PoolFee:       3000,
		TradeSizes:    []*big.Int{big.NewInt(1000000000000000000)}, // 1 ETH
		MinProfit:     decimal.NewFromFloat(10.0),
		MaxWorkers:    1,
		CacheDuration: time.Second,
	}

	manager := services.NewManager(cfg, mockCEX, mockDEX, mockListener, mockNotifier)

	// Test Data
	amountIn := cfg.TradeSizes[0] // 1 ETH
	
	// CEX OrderBook: Ask 2000 USDC
	ob := &domain.OrderBook{
		Timestamp: time.Now(),
		Asks: []domain.PriceLevel{
			{Price: decimal.NewFromFloat(2000.0), Amount: decimal.NewFromFloat(10.0)},
		},
		Bids: []domain.PriceLevel{},
	}

	// DEX Quote: 1 ETH -> 2050 USDC (Profit!)
	// 2050 * 10^6 = 2050000000
	amountOut := big.NewInt(2050000000)
	gasEstimate := big.NewInt(100000) // 100k gas
	
	pq := &domain.PriceQuote{
		Price:     decimal.NewFromBigInt(amountOut, 0),
		GasEstimate: gasEstimate,
		Timestamp: time.Now(),
	}

	// Expectations
	mockCEX.On("GetOrderBook", mock.Anything, "ETHUSDC").Return(ob, nil)
	mockDEX.On("GetQuote", mock.Anything, "0xWETH", "0xUSDC", amountIn, int64(3000)).Return(pq, nil)
	mockDEX.On("GetGasPrice", mock.Anything).Return(big.NewInt(30000000000), nil) // 30 gwei
	mockDEX.On("GetSlot0", mock.Anything, "0xWETH", "0xUSDC", int64(3000)).Return(&domain.Slot0{SqrtPriceX96: big.NewInt(0), Tick: big.NewInt(0)}, nil)
	mockNotifier.On("Broadcast", mock.Anything).Return()

	// We can't easily test the private processBlock method directly unless we export it or trigger it via Start.
	// However, for unit testing logic, it's better to test the logic method if possible.
	// But `checkArbitrageForSize` is private.
	// We can trigger `Start` and send a block to the channel.
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	blockChan := make(chan *domain.Block)
	errChan := make(chan error)
	
	mockListener.On("SubscribeNewHeads", ctx).Return((<-chan *domain.Block)(blockChan), (<-chan error)(errChan), nil)

	// Run Manager in goroutine
	go func() {
		manager.Start(ctx)
	}()

	// Send a block
	blockChan <- &domain.Block{
		Number:    big.NewInt(100),
		Timestamp: time.Now(),
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	mockCEX.AssertExpectations(t)
	mockDEX.AssertExpectations(t)
	mockNotifier.AssertExpectations(t)
}
