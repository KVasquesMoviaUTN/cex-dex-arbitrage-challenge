package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/KVasquesMoviaUTN/my-go-app/internal/core/ports"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Listener struct {
	clientURL string
}

func NewListener(clientURL string) ports.BlockchainListener {
	return &Listener{
		clientURL: clientURL,
	}
}

// SubscribeNewHeads subscribes to new block headers with reconnection logic.
func (l *Listener) SubscribeNewHeads(ctx context.Context) (<-chan *big.Int, <-chan error, error) {
	out := make(chan *big.Int)
	errChan := make(chan error)

	go func() {
		defer close(out)
		defer close(errChan)

		backoff := time.Second
		maxBackoff := 30 * time.Second

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Attempt to connect
				client, err := ethclient.DialContext(ctx, l.clientURL)
				if err != nil {
					l.logError(errChan, fmt.Errorf("failed to dial eth client: %w", err))
					time.Sleep(backoff)
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}

				// Subscribe
				headers := make(chan *types.Header)
				sub, err := client.SubscribeNewHead(ctx, headers)
				if err != nil {
					client.Close()
					l.logError(errChan, fmt.Errorf("failed to subscribe: %w", err))
					time.Sleep(backoff)
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}

				// Reset backoff on successful connection
				backoff = time.Second
				fmt.Println("Connected to Ethereum WebSocket")

				// Listen loop
				connLoop:
				for {
					select {
					case <-ctx.Done():
						sub.Unsubscribe()
						client.Close()
						return
					case err := <-sub.Err():
						l.logError(errChan, fmt.Errorf("subscription error: %w", err))
						sub.Unsubscribe()
						client.Close()
						break connLoop // Break to outer loop to reconnect
					case header := <-headers:
						select {
						case out <- header.Number:
						case <-ctx.Done():
							sub.Unsubscribe()
							client.Close()
							return
						}
					}
				}
			}
		}
	}()

	return out, errChan, nil
}

func (l *Listener) logError(ch chan<- error, err error) {
	select {
	case ch <- err:
	default:
		// Don't block if no one is listening to errors
		fmt.Printf("Blockchain Listener Error: %v\n", err)
	}
}
