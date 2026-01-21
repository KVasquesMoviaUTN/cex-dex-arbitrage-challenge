package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/domain"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/ports"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
)


const QuoterV2Address = "0x61fFE014bA17989E743c5F6cB21bF9697530B21e"


const quoterABI = `[{"inputs":[{"components":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint24","name":"fee","type":"uint24"},{"internalType":"uint160","name":"sqrtPriceLimitX96","type":"uint160"}],"internalType":"struct IQuoterV2.QuoteExactInputSingleParams","name":"params","type":"tuple"}],"name":"quoteExactInputSingle","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"},{"internalType":"uint160","name":"sqrtPriceX96After","type":"uint160"},{"internalType":"uint32","name":"initializedTicksCrossed","type":"uint32"},{"internalType":"uint256","name":"gasEstimate","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}]`

type Adapter struct {
	client    *ethclient.Client
	parsedABI abi.ABI
	poolCache sync.Map
	
	gasMu     sync.Mutex
	gasPrice  *big.Int
	gasExpiry time.Time
}

func NewAdapter(clientURL string) (ports.PriceProvider, error) {
	client, err := ethclient.Dial(clientURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ethereum node: %w", err)
	}

	parsed, err := abi.JSON(strings.NewReader(quoterABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	return &Adapter{
		client:    client,
		parsedABI: parsed,
	}, nil
}


type QuoteExactInputSingleParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	AmountIn          *big.Int
	Fee               *big.Int
	SqrtPriceLimitX96 *big.Int
}

func (a *Adapter) GetQuote(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, fee int64) (*domain.PriceQuote, error) {

	params := struct {
		TokenIn           common.Address
		TokenOut          common.Address
		AmountIn          *big.Int
		Fee               *big.Int
		SqrtPriceLimitX96 *big.Int
	}{
		TokenIn:           common.HexToAddress(tokenIn),
		TokenOut:          common.HexToAddress(tokenOut),
		AmountIn:          amountIn,
		Fee:               big.NewInt(fee),
		SqrtPriceLimitX96: big.NewInt(0),
	}


	data, err := a.parsedABI.Pack("quoteExactInputSingle", params)
	if err != nil {
		return nil, fmt.Errorf("failed to pack data: %w", err)
	}


	toAddr := common.HexToAddress(QuoterV2Address)
	msg := ethereum.CallMsg{
		To:   &toAddr,
		Data: data,
	}


	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("eth_call failed: %w", err)
	}


	// outputs: amountOut, sqrtPriceX96After, initializedTicksCrossed, gasEstimate
	unpacked, err := a.parsedABI.Unpack("quoteExactInputSingle", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}


	// Unpack returns []interface{}
	if len(unpacked) < 4 {
		return nil, fmt.Errorf("unexpected result length")
	}

	amountOut := unpacked[0].(*big.Int)
	gasEstimate := unpacked[3].(*big.Int)


	

	
	amountOutDec := decimal.NewFromBigInt(amountOut, 0)
	
	return &domain.PriceQuote{
		Price:     amountOutDec,
		GasEstimate: gasEstimate,
		Timestamp: time.Now(),
	}, nil
}

func (a *Adapter) GetGasPrice(ctx context.Context) (*big.Int, error) {
	a.gasMu.Lock()
	defer a.gasMu.Unlock()

	if a.gasPrice != nil && time.Now().Before(a.gasExpiry) {
		return a.gasPrice, nil
	}

	price, err := a.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}

	a.gasPrice = price
	a.gasExpiry = time.Now().Add(15 * time.Second)
	return price, nil
}


const FactoryAddress = "0x1F98431c8aD98523631AE4a59f267346ea31F984"


const factoryABI = `[{"inputs":[{"internalType":"address","name":"","type":"address"},{"internalType":"address","name":"","type":"address"},{"internalType":"uint24","name":"","type":"uint24"}],"name":"getPool","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}]`


const poolABI = `[{"inputs":[],"name":"slot0","outputs":[{"internalType":"uint160","name":"sqrtPriceX96","type":"uint160"},{"internalType":"int24","name":"tick","type":"int24"},{"internalType":"uint16","name":"observationIndex","type":"uint16"},{"internalType":"uint16","name":"observationCardinality","type":"uint16"},{"internalType":"uint16","name":"observationCardinalityNext","type":"uint16"},{"internalType":"uint8","name":"feeProtocol","type":"uint8"},{"internalType":"bool","name":"unlocked","type":"bool"}],"stateMutability":"view","type":"function"}]`

func (a *Adapter) GetSlot0(ctx context.Context, tokenIn, tokenOut string, fee int64) (*domain.Slot0, error) {
	poolAddr, err := a.getPoolAddress(ctx, tokenIn, tokenOut, fee)
	if err != nil {
		return nil, err
	}

	parsed, err := abi.JSON(strings.NewReader(poolABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool ABI: %w", err)
	}

	data, err := parsed.Pack("slot0")
	if err != nil {
		return nil, fmt.Errorf("failed to pack slot0: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &poolAddr,
		Data: data,
	}

	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("slot0 call failed: %w", err)
	}

	unpacked, err := parsed.Unpack("slot0", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack slot0: %w", err)
	}

	if len(unpacked) < 2 {
		return nil, fmt.Errorf("unexpected slot0 result length")
	}

	sqrtPriceX96 := unpacked[0].(*big.Int)
	tick := unpacked[1].(*big.Int)

	return &domain.Slot0{
		SqrtPriceX96: sqrtPriceX96,
		Tick:         tick,
	}, nil
}

func (a *Adapter) getPoolAddress(ctx context.Context, tokenIn, tokenOut string, fee int64) (common.Address, error) {
	t0, t1 := common.HexToAddress(tokenIn), common.HexToAddress(tokenOut)
	
	key := fmt.Sprintf("%s-%s-%d", t0.Hex(), t1.Hex(), fee)
	if val, ok := a.poolCache.Load(key); ok {
		return val.(common.Address), nil
	}
	
	parsed, err := abi.JSON(strings.NewReader(factoryABI))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to parse factory ABI: %w", err)
	}

	data, err := parsed.Pack("getPool", t0, t1, big.NewInt(fee))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to pack getPool: %w", err)
	}

	factoryAddr := common.HexToAddress(FactoryAddress)
	msg := ethereum.CallMsg{
		To:   &factoryAddr,
		Data: data,
	}

	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("getPool call failed: %w", err)
	}

	unpacked, err := parsed.Unpack("getPool", result)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to unpack getPool: %w", err)
	}

	poolAddr := unpacked[0].(common.Address)
	
	if poolAddr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("pool not found")
	}

	a.poolCache.Store(key, poolAddr)
	return poolAddr, nil
}
