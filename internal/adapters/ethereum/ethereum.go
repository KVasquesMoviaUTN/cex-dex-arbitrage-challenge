package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/domain"
	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/ports"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
)

// Uniswap V3 QuoterV2 Address
const QuoterV2Address = "0xb27308f9F90D607463bb33eA1BeBb41C27CE5AB6"

// Minimal ABI for QuoterV2 quoteExactInputSingle
// function quoteExactInputSingle(tuple(address tokenIn, address tokenOut, uint256 amountIn, uint24 fee, uint160 sqrtPriceLimitX96)) external returns (uint256 amountOut, uint160 sqrtPriceX96After, uint32 initializedTicksCrossed, uint256 gasEstimate)
const quoterABI = `[{"inputs":[{"components":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint24","name":"fee","type":"uint24"},{"internalType":"uint160","name":"sqrtPriceLimitX96","type":"uint160"}],"internalType":"struct IQuoterV2.QuoteExactInputSingleParams","name":"params","type":"tuple"}],"name":"quoteExactInputSingle","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"},{"internalType":"uint160","name":"sqrtPriceX96After","type":"uint160"},{"internalType":"uint32","name":"initializedTicksCrossed","type":"uint32"},{"internalType":"uint256","name":"gasEstimate","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}]`

type Adapter struct {
	client *ethclient.Client
	parsedABI abi.ABI
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

// QuoteExactInputSingleParams struct matching the ABI tuple
type QuoteExactInputSingleParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	AmountIn          *big.Int
	Fee               *big.Int // uint24 in ABI, but go-ethereum uses big.Int for numbers usually, or uint32/uint64. Let's check packing.
	SqrtPriceLimitX96 *big.Int
}

func (a *Adapter) GetQuote(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, fee int64) (*domain.PriceQuote, error) {
	// Prepare the input struct
	params := struct {
		TokenIn           common.Address
		TokenOut          common.Address
		AmountIn          *big.Int
		Fee               *big.Int // Using big.Int for safety with packing, though it's uint24
		SqrtPriceLimitX96 *big.Int
	}{
		TokenIn:           common.HexToAddress(tokenIn),
		TokenOut:          common.HexToAddress(tokenOut),
		AmountIn:          amountIn,
		Fee:               big.NewInt(fee),
		SqrtPriceLimitX96: big.NewInt(0),
	}

	// Pack the input arguments
	data, err := a.parsedABI.Pack("quoteExactInputSingle", params)
	if err != nil {
		return nil, fmt.Errorf("failed to pack data: %w", err)
	}

	// Create the call message
	toAddr := common.HexToAddress(QuoterV2Address)
	msg := ethereum.CallMsg{
		To:   &toAddr,
		Data: data,
	}

	// Execute the call (eth_call)
	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("eth_call failed: %w", err)
	}

	// Unpack the output
	// outputs: amountOut, sqrtPriceX96After, initializedTicksCrossed, gasEstimate
	unpacked, err := a.parsedABI.Unpack("quoteExactInputSingle", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	// Extract values
	// Unpack returns []interface{}
	if len(unpacked) < 4 {
		return nil, fmt.Errorf("unexpected result length")
	}

	amountOut := unpacked[0].(*big.Int)
	gasEstimate := unpacked[3].(*big.Int)

	// Calculate effective price: AmountOut / AmountIn
	// Note: This is raw units. Decimal adjustment happens in the Manager.
	// But for the PriceQuote struct, let's store the raw ratio or maybe just the raw amounts?
	// The domain struct expects decimal.Decimal. Let's convert.
	
	// We need to know decimals to give a "human readable" price here, but the adapter might not know decimals.
	// However, the prompt says "Handle the 18 decimals (ETH) vs 6 decimals (USDC) conversion correctly using math/big."
	// Usually this logic resides in the core service which knows about the tokens. 
	// But let's return the raw amountOut as a Decimal for now, or maybe the implied price?
	// Let's stick to returning the raw output amount in the PriceQuote for now, 
	// OR better, let's make PriceQuote hold the AmountOut and let the service calculate the ratio.
	// Wait, the domain struct has `Price decimal.Decimal`. 
	// If I don't know the decimals here, I can't calculate the human readable price.
	// I will assume the caller handles decimal adjustment if I just return the raw ratio?
	// No, the prompt explicitly asks to handle conversion.
	// I will assume standard decimals for ETH (18) and USDC (6) for this specific pair as per prompt context.
	// But this is a generic adapter. 
	// Let's modify the PriceQuote domain to include AmountOut instead of just Price, 
	// or let's just return the AmountOut in the Price field (as a raw value) and let the manager handle it.
	// Actually, looking at the prompt: "Fetch ETH/USDC price... Handle the 18 decimals... conversion correctly"
	// I will do the conversion here if I know which is which.
	// But `GetQuote` takes generic token addresses.
	// I will return the `AmountOut` as a decimal in the `Price` field for now (naming it Price is slightly misleading if it's an amount, but let's assume Price = AmountOut for 1 unit of input? No, input is variable).
	// Let's change the Domain to be more flexible or just return the AmountOut.
	// I'll stick to the plan: The Manager will know the decimals. 
	// I will return the AmountOut in the Price field (as a decimal) and the Manager will do the math: (AmountOut / 10^OutDec) / (AmountIn / 10^InDec).
	
	amountOutDec := decimal.NewFromBigInt(amountOut, 0)
	
	return &domain.PriceQuote{
		Price:     amountOutDec, // This is actually AmountOut. The manager should interpret this.
		GasEstimate: gasEstimate,
		Timestamp: time.Now(),
	}, nil
}
