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
	"golang.org/x/sync/errgroup"
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

	// Concurrent Fetching using errgroup
	g, ctx := errgroup.WithContext(ctx)
	
	var ob *domain.OrderBook

	// 1. Fetch CEX Orderbook
	g.Go(func() error {
		var err error
		ob, err = m.cex.GetOrderBook(ctx, m.cfg.Symbol)
		if err != nil {
			return fmt.Errorf("failed to fetch CEX: %w", err)
		}
		return nil
	})

	// 2. Fetch DEX Quotes (Parallel for each trade size)
	// We need to store results safely.
	// Since we are just checking arbitrage inside the loop in the original code,
	// we can either:
	// A) Fetch all quotes first, then check.
	// B) Check inside the goroutine (but we need CEX ob first).
	//
	// Better approach:
	// Fetch CEX and ALL DEX quotes in parallel.
	// Then once all data is ready, run the logic.
	
	type quoteResult struct {
		amountIn *big.Int
		quote    *domain.PriceQuote
	}
	quoteResults := make([]quoteResult, len(m.cfg.TradeSizes))

	for i, amountIn := range m.cfg.TradeSizes {
		i, amountIn := i, amountIn // capture loop variables
		g.Go(func() error {
			pq, err := m.dex.GetQuote(ctx, m.cfg.TokenInAddr, m.cfg.TokenOutAddr, amountIn, m.cfg.PoolFee)
			if err != nil {
				slog.Error("Error fetching DEX quote", "amount", amountIn, "error", err)
				return nil // Don't fail the whole group, just skip this size
			}
			quoteResults[i] = quoteResult{amountIn: amountIn, quote: pq}
			return nil
		})
	}

	// Wait for all fetches
	if err := g.Wait(); err != nil {
		slog.Error("Error during data fetch", "error", err)
		return
	}

	// 3. Analyze Opportunities (CPU bound, fast)
	for _, res := range quoteResults {
		if res.quote == nil {
			continue
		}
		m.checkArbitrageWithData(ctx, ob, res.amountIn, res.quote)
	}
}

// checkArbitrageWithData is a helper to keep logic clean after parallel fetch
func (m *Manager) checkArbitrageWithData(ctx context.Context, ob *domain.OrderBook, amountIn *big.Int, pq *domain.PriceQuote) {
	amountInDec := decimal.NewFromBigInt(amountIn, -m.cfg.TokenInDec)
	amountOutDec := pq.Price.Mul(decimal.NewFromFloat(1).Div(decimal.New(1, m.cfg.TokenOutDec)))
	
	dexEffectivePrice := amountOutDec.Div(amountInDec)

	// Calculate effective ask price on CEX for this volume.
	cexEffectivePrice, ok := ob.CalculateEffectivePrice("buy", amountInDec)
	if !ok {
		return
	}

	// Profit Calculation
	cexFeeRate := decimal.NewFromFloat(0.001)
	cexCost := cexEffectivePrice.Mul(amountInDec).Mul(decimal.NewFromFloat(1).Add(cexFeeRate))
	
	gasUsed := decimal.NewFromBigInt(pq.GasEstimate, 0)
	gasPriceGwei := decimal.NewFromFloat(30)
	gasPriceETH := gasPriceGwei.Mul(decimal.NewFromFloat(1e-9))
	ethPriceUSDC := cexEffectivePrice
	gasCostUSDC := gasUsed.Mul(gasPriceETH).Mul(ethPriceUSDC)
	
	dexRevenue := amountOutDec.Sub(gasCostUSDC)
	
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
