package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/adapters/binance"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/adapters/blockchain"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/adapters/ethereum"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/services"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	go func() {
		port := viper.GetString("METRICS_PORT")
		addr := ":" + port
		http.Handle("/metrics", promhttp.Handler())
		slog.Info("Starting metrics server", "port", port)
		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	viper.SetDefault("ETH_NODE_WS", "wss://mainnet.infura.io/ws/v3/YOUR_KEY")
	viper.SetDefault("ETH_NODE_HTTP", "https://mainnet.infura.io/v3/YOUR_KEY")
	viper.SetDefault("SYMBOL", "ETHUSDC")
	viper.SetDefault("TOKEN_IN", "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2") // WETH
	viper.SetDefault("TOKEN_OUT", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48") // USDC
	viper.SetDefault("TOKEN_IN_DEC", 18)
	viper.SetDefault("TOKEN_OUT_DEC", 6)
	viper.SetDefault("POOL_FEE", 3000) // 0.3%
	viper.SetDefault("TRADE_SIZES", "1000000000000000000,10000000000000000000") // 1 ETH, 10 ETH
	viper.SetDefault("MIN_PROFIT", "10.0") // 10 USDC
	viper.SetDefault("MAX_WORKERS", 5)
	viper.SetDefault("METRICS_PORT", "8085")
	viper.SetDefault("BINANCE_API_URL", "https://api.binance.com/api/v3")

	viper.AutomaticEnv()

	cfg := services.Config{
		Symbol:        viper.GetString("SYMBOL"),
		TokenInAddr:   viper.GetString("TOKEN_IN"),
		TokenOutAddr:  viper.GetString("TOKEN_OUT"),
		TokenInDec:    viper.GetInt32("TOKEN_IN_DEC"),
		TokenOutDec:   viper.GetInt32("TOKEN_OUT_DEC"),
		PoolFee:       viper.GetInt64("POOL_FEE"),
		MaxWorkers:    viper.GetInt("MAX_WORKERS"),
		CacheDuration: 10 * time.Second,
	}

	tradeSizesStr := viper.GetString("TRADE_SIZES")
	tradeSizes, err := parseTradeSizes(tradeSizesStr)
	if err != nil {
		slog.Warn("Failed to parse some trade sizes", "error", err)
	}
	if len(tradeSizes) == 0 {
		log.Fatal("No valid TRADE_SIZES configured")
	}
	cfg.TradeSizes = tradeSizes

	minProfitStr := viper.GetString("MIN_PROFIT")
	minProfit, err := decimal.NewFromString(minProfitStr)
	if err != nil {
		log.Fatalf("Invalid MIN_PROFIT: %s", minProfitStr)
	}
	cfg.MinProfit = minProfit

	binanceURL := viper.GetString("BINANCE_API_URL")
	slog.Info("Initializing Binance Adapter", "url", binanceURL)
	cexAdapter := binance.NewAdapter(binanceURL)

	dexAdapter, err := ethereum.NewAdapter(viper.GetString("ETH_NODE_HTTP"))
	if err != nil {
		log.Fatalf("Failed to create Ethereum adapter: %v", err)
	}

	listener := blockchain.NewListener(viper.GetString("ETH_NODE_WS"))

	manager := services.NewManager(cfg, cexAdapter, dexAdapter, listener)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	if err := manager.Start(ctx); err != nil {
		log.Printf("Manager finished with error: %v", err)
	} else {
		log.Println("Manager finished successfully")
	}
}

func parseTradeSizes(input string) ([]*big.Int, error) {
	var sizes []*big.Int
	parts := strings.Split(input, ",")
	
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		
		val, ok := new(big.Int).SetString(p, 10)
		if !ok {
			return sizes, fmt.Errorf("invalid trade size value: %s", p)
		}
		sizes = append(sizes, val)
	}
	
	return sizes, nil
}
