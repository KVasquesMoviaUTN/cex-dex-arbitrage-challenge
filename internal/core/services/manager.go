package services

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/ports"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/observability"
	"github.com/shopspring/decimal"
)

// Config holds the configuration for the Manager.
type Config struct {
	TokenInAddr   string
	TokenOutAddr  string
	TokenInDec    int32
	TokenOutDec   int32
	Symbol        string
	PoolFee       int64
	TradeSizes    []*big.Int // List of trade sizes in Wei
	MinProfit     decimal.Decimal
	MaxWorkers    int
	CacheDuration time.Duration
}

// Manager orchestrates the arbitrage bot.
type Manager struct {
	cfg        Config
	cex        ports.ExchangeAdapter
	dex        ports.PriceProvider
	listener   ports.BlockchainListener
	
	// Caching
	mu         sync.RWMutex
	lastBlock  *big.Int
	
	// Worker Pool
	sem        chan struct{}
}

func NewManager(cfg Config, cex ports.ExchangeAdapter, dex ports.PriceProvider, listener ports.BlockchainListener) *Manager {
	return &Manager{
		cfg:      cfg,
		cex:      cex,
		dex:      dex,
		listener: listener,
		sem:      make(chan struct{}, cfg.MaxWorkers),
	}
}

// Start begins the bot's operation.
func (m *Manager) Start(ctx context.Context) error {
	blockChan, errChan, err := m.listener.SubscribeNewHeads(ctx)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	slog.Info("Bot started. Waiting for blocks...")

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errChan:
			slog.Error("Listener error", "error", err)
		case blockNum := <-blockChan:
			observability.BlocksProcessed.Inc()
			// Non-blocking send to worker pool (drop if full, or block? Prompt says "limit concurrent processing")
			// If blocks come too fast, we might want to skip.
			select {
			case m.sem <- struct{}{}:
				observability.ActiveWorkers.Inc()
				go func(bn *big.Int) {
					defer func() { 
						<-m.sem 
						observability.ActiveWorkers.Dec()
					}()
					m.processBlock(ctx, bn)
				}(blockNum)
			default:
				slog.Warn("Worker pool full, skipping block", "block", blockNum)
			}
		}
	}
}

func (m *Manager) processBlock(ctx context.Context, blockNum *big.Int) {
	// Check cache/deduplication
	m.mu.RLock()
	if m.lastBlock != nil && m.lastBlock.Cmp(blockNum) == 0 {
		m.mu.RUnlock()
		return // Already processed this block
	}
	m.mu.RUnlock()

	m.mu.Lock()
	m.lastBlock = blockNum
	m.mu.Unlock()

	slog.Info("Processing Block", "block", blockNum)

	// Concurrent Fetching
	var (
		ob  *domain.OrderBook
		errCEX error
		wg sync.WaitGroup
	)

	wg.Add(1)

	// Fetch CEX
	go func() {
		defer wg.Done()
		ob, errCEX = m.cex.GetOrderBook(ctx, m.cfg.Symbol)
	}()

	wg.Wait()

	if errCEX != nil {
		slog.Error("Error fetching CEX", "error", errCEX)
		return
	}

	// Check all trade sizes
	for _, amountIn := range m.cfg.TradeSizes {
		m.checkArbitrageForSize(ctx, ob, amountIn)
	}
}

func (m *Manager) checkArbitrageForSize(ctx context.Context, ob *domain.OrderBook, amountIn *big.Int) {
	// 1. Get DEX Price (Sell ETH for USDC)
	// We need to fetch the quote specifically for this amount because of slippage on DEX
	pq, err := m.dex.GetQuote(ctx, m.cfg.TokenInAddr, m.cfg.TokenOutAddr, amountIn, m.cfg.PoolFee)
	if err != nil {
		slog.Error("Error fetching DEX quote", "amount", amountIn, "error", err)
		return
	}

	amountInDec := decimal.NewFromBigInt(amountIn, -m.cfg.TokenInDec) // e.g. 10 ETH
	amountOutDec := pq.Price.Mul(decimal.NewFromFloat(1).Div(decimal.New(1, m.cfg.TokenOutDec))) // Raw AmountOut * 10^-6
	
	dexEffectivePrice := amountOutDec.Div(amountInDec) // Price of 1 ETH in USDC (effective)

	// 2. Get CEX Price (Buy ETH with USDC)
	// We need to buy `amountIn` ETH.
	// Calculate effective ask price on CEX for this volume.
	cexEffectivePrice, ok := ob.CalculateEffectivePrice("buy", amountInDec)
	if !ok {
		// Not enough liquidity on CEX
		return
	}

	// 3. Profit Calculation
	// Direction: Buy CEX -> Sell DEX
	
	// Costs & Fees
	// CEX Fee: 0.1%
	cexFeeRate := decimal.NewFromFloat(0.001)
	cexCost := cexEffectivePrice.Mul(amountInDec).Mul(decimal.NewFromFloat(1).Add(cexFeeRate)) // Total USDC spent
	
	// DEX Revenue
	// Output from Quoter is already net of pool fee (0.3%).
	// We just need to subtract Gas.
	// Gas Cost in USDC. Estimate: GasUsed * GasPrice * ETHPrice.
	// Let's assume GasPrice = 30 Gwei (standard) and ETH Price = CEX Price.
	gasUsed := decimal.NewFromBigInt(pq.GasEstimate, 0)
	gasPriceGwei := decimal.NewFromFloat(30) // 30 Gwei
	gasPriceETH := gasPriceGwei.Mul(decimal.NewFromFloat(1e-9))
	ethPriceUSDC := cexEffectivePrice
	gasCostUSDC := gasUsed.Mul(gasPriceETH).Mul(ethPriceUSDC)
	
	dexRevenue := amountOutDec.Sub(gasCostUSDC)
	
	// Profit
	profit := dexRevenue.Sub(cexCost)
	
	if profit.GreaterThan(m.cfg.MinProfit) {
		observability.ArbitrageOpsFound.Inc()
		profitFloat, _ := profit.Float64()
		observability.ArbitrageProfit.WithLabelValues("USDC").Add(profitFloat)
		
		m.printReport(amountInDec, cexEffectivePrice, dexEffectivePrice, profit, "CEX -> DEX")
	}
}

func (m *Manager) printReport(amount, cexPrice, dexPrice, profit decimal.Decimal, direction string) {
	// Structured log for machine consumption
	slog.Info("Arbitrage Opportunity Detected",
		"timestamp", time.Now().UTC(),
		"direction", direction,
		"trade_size_eth", amount.StringFixed(2),
		"cex_price", cexPrice.StringFixed(2),
		"dex_price", dexPrice.StringFixed(2),
		"estimated_profit_usdc", profit.StringFixed(2),
	)

	// Human readable output
	fmt.Println("=== ARBITRAGE OPPORTUNITY DETECTED ===")
	fmt.Printf("Timestamp: %s\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Direction: %s\n", direction)
	fmt.Printf("Trade Size: %s ETH\n", amount.StringFixed(2))
	fmt.Printf("CEX Price: $%s (effective)\n", cexPrice.StringFixed(2))
	fmt.Printf("DEX Price: $%s (effective)\n", dexPrice.StringFixed(2))
	fmt.Printf("Estimated Profit: $%s\n", profit.StringFixed(2))
	fmt.Println("Execution Steps:")
	if direction == "CEX -> DEX" {
		fmt.Printf("1. Buy %s ETH on Binance at avg price $%s\n", amount.StringFixed(2), cexPrice.StringFixed(2))
		fmt.Printf("2. Transfer ETH to wallet\n")
		fmt.Printf("3. Sell %s ETH on Uniswap V3\n", amount.StringFixed(2))
	}
	fmt.Println("======================================")
}
