package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	BlocksProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "arbitrage_blocks_processed_total",
		Help: "The total number of blocks processed",
	})

	ArbitrageOpsFound = promauto.NewCounter(prometheus.CounterOpts{
		Name: "arbitrage_opportunities_found_total",
		Help: "The total number of arbitrage opportunities found",
	})

	ArbitrageProfit = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "arbitrage_profit_total",
		Help: "The total profit from arbitrage opportunities",
	}, []string{"asset"})

	ActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "arbitrage_active_workers",
		Help: "The number of active workers processing blocks",
	})
)
