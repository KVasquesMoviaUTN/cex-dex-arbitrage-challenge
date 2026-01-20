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
		return
	}
	m.mu.RUnlock()

	m.mu.Lock()
	m.lastBlock = blockNum
	m.mu.Unlock()

	slog.Info("new block", "height", blockNum)

	g, ctx := errgroup.WithContext(ctx)
	
	var ob *domain.OrderBook

	g.Go(func() error {
		var err error
		ob, err = m.cex.GetOrderBook(ctx, m.cfg.Symbol)
		if err != nil {
			return fmt.Errorf("cex fetch failed: %w", err)
		}
		return nil
	})

	type quoteResult struct {
		amt   *big.Int
		quote *domain.PriceQuote
	}
	quoteResults := make([]quoteResult, len(m.cfg.TradeSizes))

	for i, size := range m.cfg.TradeSizes {
		i, size := i, size
		g.Go(func() error {
			pq, err := m.dex.GetQuote(ctx, m.cfg.TokenInAddr, m.cfg.TokenOutAddr, size, m.cfg.PoolFee)
			if err != nil {
				slog.Error("dex quote failed", "size", size, "err", err)
				return nil
			}
			quoteResults[i] = quoteResult{amt: size, quote: pq}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.Error("data fetch failed", "err", err)
		return
	}

	for _, res := range quoteResults {
		if res.quote == nil {
			continue
		}
		m.checkArbitrageWithData(ctx, blockNum, ob, res.amt, res.quote)
	}
}

func (m *Manager) checkArbitrageWithData(ctx context.Context, blockNum *big.Int, ob *domain.OrderBook, amountIn *big.Int, pq *domain.PriceQuote) {
	amtIn := decimal.NewFromBigInt(amountIn, -m.cfg.TokenInDec)
	amtOut := pq.Price.Mul(decimal.NewFromFloat(1).Div(decimal.New(1, m.cfg.TokenOutDec)))
	
	dexPrice := amtOut.Div(amtIn)

	cexPrice, ok := ob.CalculateEffectivePrice("buy", amtIn)
	if !ok {
		slog.Info(fmt.Sprintf("[DEBUG] Block %s: Size %s | CEX Price Unavailable", blockNum, amtIn))
		return
	}

	spread := dexPrice.Sub(cexPrice).Div(cexPrice).Mul(decimal.NewFromFloat(100))
	slog.Info(fmt.Sprintf("[DEBUG] Block %s: Binance %s | Uniswap %s | Spread %s%% | Size %s", 
		blockNum, cexPrice.StringFixed(2), dexPrice.StringFixed(2), spread.StringFixed(2), amtIn.StringFixed(2)))

	cexFee := decimal.NewFromFloat(0.001)
	cexCost := cexPrice.Mul(amtIn).Mul(decimal.NewFromFloat(1).Add(cexFee))
	
	gasUsed := decimal.NewFromBigInt(pq.GasEstimate, 0)
	gasPrice := decimal.NewFromFloat(30).Mul(decimal.NewFromFloat(1e-9)) // 30 gwei
	gasCost := gasUsed.Mul(gasPrice).Mul(cexPrice) // value gas in USDC
	
	netDex := amtOut.Sub(gasCost)
	profit := netDex.Sub(cexCost)
	
	if profit.GreaterThan(m.cfg.MinProfit) {
		observability.ArbitrageOpsFound.Inc()
		p, _ := profit.Float64()
		observability.ArbitrageProfit.WithLabelValues("USDC").Add(p)
		
		m.printReport(amtIn, cexPrice, dexPrice, profit, "CEX -> DEX")
	}
}

func (m *Manager) printReport(amount, cexPrice, dexPrice, profit decimal.Decimal, direction string) {
	slog.Info("arb opportunity",
		"ts", time.Now().UTC(),
		"dir", direction,
		"size", amount.StringFixed(2),
		"cex", cexPrice.StringFixed(2),
		"dex", dexPrice.StringFixed(2),
		"profit", profit.StringFixed(2),
	)

	fmt.Println(">>> ARB FOUND <<<")
	fmt.Printf("Time: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Printf("Dir:  %s\n", direction)
	fmt.Printf("Size: %s ETH\n", amount.StringFixed(2))
	fmt.Printf("CEX:  $%s\n", cexPrice.StringFixed(2))
	fmt.Printf("DEX:  $%s\n", dexPrice.StringFixed(2))
	fmt.Printf("Est. Profit: $%s\n", profit.StringFixed(2))
	fmt.Println("---------------------")
}
