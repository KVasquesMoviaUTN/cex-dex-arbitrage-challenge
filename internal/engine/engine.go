package engine

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/adapters/binance"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/adapters/blockchain"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/adapters/ethereum"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/adapters/kraken"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/adapters/okx"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/adapters/websocket"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/ports"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/services"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	services.Config
	EthNodeWS     string
	EthNodeHTTP   string
	MetricsPort   string
	CEXProvider   string
	BinanceAPIURL string
}

type Engine struct {
	cfg      Config
	manager  *services.Manager
	notifier *websocket.Server
}

func New(cfg Config) (*Engine, error) {
	cex := createCEXAdapter(cfg.CEXProvider, cfg.BinanceAPIURL)
	slog.Info("Using CEX provider", "provider", cfg.CEXProvider)

	dex, err := ethereum.NewAdapter(cfg.EthNodeHTTP)
	if err != nil {
		return nil, fmt.Errorf("failed to create DEX adapter: %w", err)
	}

	listener := blockchain.NewListener(cfg.EthNodeWS)
	notifier := websocket.NewServer()

	manager := services.NewManager(cfg.Config, cex, dex, listener, notifier)

	return &Engine{
		cfg:      cfg,
		manager:  manager,
		notifier: notifier,
	}, nil
}

func (e *Engine) Run(ctx context.Context) error {

	go func() {
		addr := ":" + e.cfg.MetricsPort
		http.Handle("/metrics", promhttp.Handler())
		slog.Info("Starting metrics server", "port", e.cfg.MetricsPort)
		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("Metrics server failed", "error", err)
		}
	}()


	go func() {
		e.notifier.Start(":8080")
	}()


	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Shutdown signal received")
		cancel()
	}()

	slog.Info("Starting arbitrage bot")
	if err := e.manager.Start(ctx); err != nil {
		return fmt.Errorf("manager failed: %w", err)
	}

	return nil
}

func createCEXAdapter(provider, binanceURL string) ports.ExchangeAdapter {
	switch strings.ToLower(provider) {
	case "kraken":
		return kraken.NewAdapter()
	case "okx":
		return okx.NewAdapter()
	case "binance":
		fallthrough
	default:
		return binance.NewAdapter(binanceURL)
	}
}

func ParseTradeSizes(s string) ([]*big.Int, error) {
	parts := strings.Split(s, ",")
	sizes := make([]*big.Int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		size, ok := new(big.Int).SetString(p, 10)
		if !ok {
			return sizes, fmt.Errorf("invalid trade size: %s", p)
		}
		sizes = append(sizes, size)
	}
	return sizes, nil
}
