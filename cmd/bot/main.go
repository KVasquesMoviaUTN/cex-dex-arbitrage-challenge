package main

import (
	"context"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/adapters/binance"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/adapters/blockchain"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/adapters/ethereum"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/services"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func main() {
	// 1. Configuration
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
	// Split by comma and parse
	// For simplicity in this scaffold, let's just parse the default string or assume comma separated
	// In a real app, we'd use a helper.
	// Let's just hardcode a few for now if parsing is complex, or do a simple split.
	// Actually, viper.GetStringSlice might work if env var is set correctly, but defaults are tricky.
	// Let's just parse the string manually.
	
	sizes := strings.Split(tradeSizesStr, ",")
	for _, s := range sizes {
		amt, ok := new(big.Int).SetString(strings.TrimSpace(s), 10)
		if ok {
			cfg.TradeSizes = append(cfg.TradeSizes, amt)
		}
	}

	minProfitStr := viper.GetString("MIN_PROFIT")
	minProfit, err := decimal.NewFromString(minProfitStr)
	if err != nil {
		log.Fatalf("Invalid MIN_PROFIT: %s", minProfitStr)
	}
	cfg.MinProfit = minProfit

	// 2. Adapters
	cexAdapter := binance.NewAdapter()

	dexAdapter, err := ethereum.NewAdapter(viper.GetString("ETH_NODE_HTTP"))
	if err != nil {
		log.Fatalf("Failed to create Ethereum adapter: %v", err)
	}

	listener := blockchain.NewListener(viper.GetString("ETH_NODE_WS"))

	// 3. Manager
	manager := services.NewManager(cfg, cexAdapter, dexAdapter, listener)

	// 4. Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// 5. Start
	if err := manager.Start(ctx); err != nil {
		log.Printf("Manager finished with error: %v", err)
	} else {
		log.Println("Manager finished successfully")
	}
}
