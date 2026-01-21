package services

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/domain"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/ports"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/observability"
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
	TradeSizes    []*big.Int
	MinProfit     decimal.Decimal
	MaxWorkers    int
	CacheDuration time.Duration
}


type Manager struct {
	cfg        Config
	cex        ports.ExchangeAdapter
	dex        ports.PriceProvider
	listener   ports.BlockchainListener
	notifier   ports.NotificationService
	
	mu         sync.RWMutex
	lastBlock  *big.Int
	
	sem        chan struct{}
}

func NewManager(cfg Config, cex ports.ExchangeAdapter, dex ports.PriceProvider, listener ports.BlockchainListener, notifier ports.NotificationService) *Manager {
	return &Manager{
		cfg:      cfg,
		cex:      cex,
		dex:      dex,
		listener: listener,
		notifier: notifier,
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
		case block := <-blockChan:
			observability.BlocksProcessed.Inc()
			select {
			case m.sem <- struct{}{}:
				observability.ActiveWorkers.Inc()
				go func(b *domain.Block) {
					defer func() { 
						<-m.sem 
						observability.ActiveWorkers.Dec()
					}()
					m.processBlock(ctx, b)
				}(block)
			default:
				slog.Warn("Worker pool full, skipping block", "block", block.Number)
			}
		}
	}
}

func (m *Manager) processBlock(ctx context.Context, block *domain.Block) {

	if time.Since(block.Timestamp) > 20*time.Second {
		slog.Warn("Circuit Breaker: Skipping stale block", "block", block.Number, "age", time.Since(block.Timestamp))
		return
	}

	blockNum := block.Number
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
	
	slog.Info("new block", "height", blockNum)
	
	m.notifier.Broadcast(domain.ArbitrageEvent{
		Type:        "HEARTBEAT",
		BlockNumber: blockNum.Uint64(),
		Timestamp:   time.Now(),
	})

	g, ctx := errgroup.WithContext(ctx)
	
	var ob *domain.OrderBook
	var gasPrice *big.Int
	var slot0 *domain.Slot0

	g.Go(func() error {
		var err error
		ob, err = m.cex.GetOrderBook(ctx, m.cfg.Symbol)
		if err != nil {
			return fmt.Errorf("cex fetch failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		gasPrice, err = m.dex.GetGasPrice(ctx)
		if err != nil {
			slog.Warn("failed to fetch gas price, using default", "err", err)
			gasPrice = big.NewInt(30000000000) // 30 gwei default
		}
		return nil
	})

	g.Go(func() error {
		var err error
		slot0, err = m.dex.GetSlot0(ctx, m.cfg.TokenInAddr, m.cfg.TokenOutAddr, m.cfg.PoolFee)
		if err != nil {
			slog.Warn("failed to fetch slot0, skipping pre-flight check", "err", err)
		}
		return nil
	})

	type quoteResult struct {
		amt   *big.Int
		quote *domain.PriceQuote
	}
	quoteResults := make([]quoteResult, len(m.cfg.TradeSizes))


	if slot0 != nil && ob != nil && len(ob.Asks) > 0 {
		slog.Info("Pre-flight check available", "slot0_tick", slot0.Tick)
	}

	for i, size := range m.cfg.TradeSizes {
		i, size := i, size // capture loop variables
		g.Go(func() error {
			quote, err := m.dex.GetQuote(ctx, m.cfg.TokenInAddr, m.cfg.TokenOutAddr, size, m.cfg.PoolFee)
			if err != nil {
				return fmt.Errorf("dex quote failed for size %s: %w", size, err)
			}
			quoteResults[i] = quoteResult{amt: size, quote: quote}
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
		m.checkArbitrageWithData(ctx, blockNum, ob, res.amt, res.quote, gasPrice)
	}
}

func (m *Manager) checkArbitrageWithData(ctx context.Context, blockNum *big.Int, ob *domain.OrderBook, amountIn *big.Int, pq *domain.PriceQuote, gasPriceWei *big.Int) {
	amtIn := decimal.NewFromBigInt(amountIn, -m.cfg.TokenInDec)
	amtOut := pq.Price.Mul(decimal.NewFromFloat(1).Div(decimal.New(1, m.cfg.TokenOutDec)))
	
	dexPrice := amtOut.Div(amtIn)

	cexPrice, ok := ob.CalculateEffectivePrice("buy", amtIn)
	if !ok {
		slog.Info(fmt.Sprintf("[DEBUG] Block %s: Size %s | CEX Price Unavailable", blockNum, amtIn))
		return
	}

	spread := dexPrice.Sub(cexPrice).Div(cexPrice).Mul(decimal.NewFromFloat(100))
	slog.Info("Market analysis complete",
		"block", blockNum,
		"binance_price", cexPrice.StringFixed(2),
		"uniswap_price", dexPrice.StringFixed(2),
		"spread_pct", spread.StringFixed(2),
		"status", "no_opportunity",
		"size", amtIn.StringFixed(2),
		"size", amtIn.StringFixed(2),
	)

	cexFee := decimal.NewFromFloat(0.001)
	cexCost := cexPrice.Mul(amtIn).Mul(decimal.NewFromFloat(1).Add(cexFee))
	
	gasUsed := decimal.NewFromBigInt(pq.GasEstimate, 0)
	

	gasPriceEth := decimal.NewFromBigInt(gasPriceWei, -18)
	gasCost := gasUsed.Mul(gasPriceEth).Mul(cexPrice)
	
	netDex := amtOut.Sub(gasCost)
	profit := netDex.Sub(cexCost)

	cexPriceFloat, _ := cexPrice.Float64()
	dexPriceFloat, _ := dexPrice.Float64()
	spreadFloat, _ := spread.Float64()
	profitFloat, _ := profit.Float64()
	gasCostFloat, _ := gasCost.Float64()

	m.notifier.Broadcast(domain.ArbitrageEvent{
		Type:        "OPPORTUNITY",
		BlockNumber: blockNum.Uint64(),
		Timestamp:   time.Now(),
		Data: &domain.TradeData{
			CexPrice:        cexPriceFloat,
			DexPrice:        dexPriceFloat,
			SpreadPct:       spreadFloat,
			EstimatedProfit: profitFloat,
			GasCost:         gasCostFloat,
			Symbol:          m.cfg.Symbol,
		},
	})
	
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
