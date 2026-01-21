package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestGetQuote(t *testing.T) {
	// Mock JSON-RPC Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Mock eth_call response
		// We need to return a hex string representing the output tuple:
		// (amountOut, sqrtPriceX96After, initializedTicksCrossed, gasEstimate)
		// Let's say:
		// amountOut = 2000000000 (2000 USDC)
		// sqrtPriceX96After = 0
		// initializedTicksCrossed = 0
		// gasEstimate = 100000

		amountOut := common.LeftPadBytes(big.NewInt(2000000000).Bytes(), 32)
		sqrtPrice := common.LeftPadBytes(big.NewInt(0).Bytes(), 32)
		ticks := common.LeftPadBytes(big.NewInt(0).Bytes(), 32)
		gas := common.LeftPadBytes(big.NewInt(100000).Bytes(), 32)

		var result []byte
		result = append(result, amountOut...)
		result = append(result, sqrtPrice...)
		result = append(result, ticks...)
		result = append(result, gas...)

		hexResult := hexutil.Encode(result)

		response := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":"%s"}`, hexResult)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, response)
	}))
	defer ts.Close()

	// Initialize Adapter
	adapter, err := NewAdapter(ts.URL)
	assert.NoError(t, err)

	// Test
	ctx := context.Background()
	amountIn := big.NewInt(1000000000000000000) // 1 ETH
	quote, err := adapter.GetQuote(ctx, "0xWETH", "0xUSDC", amountIn, 3000)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, quote)
	
	// Check Price (AmountOut)
	expectedPrice := decimal.NewFromInt(2000000000)
	assert.True(t, quote.Price.Equal(expectedPrice), "Price mismatch")

	// Check Gas Estimate
	expectedGas := big.NewInt(100000)
	assert.Equal(t, expectedGas, quote.GasEstimate, "Gas estimate mismatch")
}

func TestGetGasPrice(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock eth_gasPrice response
		// 30 Gwei = 30000000000 Wei
		response := `{"jsonrpc":"2.0","id":1,"result":"0x6fc23ac00"}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, response)
	}))
	defer ts.Close()

	adapter, err := NewAdapter(ts.URL)
	assert.NoError(t, err)

	gasPrice, err := adapter.GetGasPrice(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(30000000000), gasPrice)
}

func TestGetGasPrice_Caching(t *testing.T) {
	reqCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		response := `{"jsonrpc":"2.0","id":1,"result":"0x6fc23ac00"}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, response)
	}))
	defer ts.Close()

	adapter, err := NewAdapter(ts.URL)
	assert.NoError(t, err)

	// First call
	_, err = adapter.GetGasPrice(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, reqCount)

	// Second call (immediate) - should hit cache
	_, err = adapter.GetGasPrice(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, reqCount)
}
