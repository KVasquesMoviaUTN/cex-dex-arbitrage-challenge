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

func (l *Listener) SubscribeNewHeads(ctx context.Context) (<-chan *big.Int, <-chan error, error) {
	out := make(chan *big.Int)
	errChan := make(chan error)

	go func() {
		defer close(out)
		defer close(errChan)

		backoff := time.Second
		maxBackoff := 30 * time.Second

		heartbeatInterval := 30 * time.Second
		timer := time.NewTimer(heartbeatInterval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				client, err := ethclient.DialContext(ctx, l.clientURL)
				if err != nil {
					l.logError(errChan, fmt.Errorf("dial failed: %w", err))
					time.Sleep(backoff)
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}

				headers := make(chan *types.Header)
				sub, err := client.SubscribeNewHead(ctx, headers)
				if err != nil {
					client.Close()
					l.logError(errChan, fmt.Errorf("sub failed: %w", err))
					time.Sleep(backoff)
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}

				backoff = time.Second
				fmt.Println("ws connected")

				connLoop:
				for {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(heartbeatInterval)

					select {
					case <-ctx.Done():
						sub.Unsubscribe()
						client.Close()
						return
					case err := <-sub.Err():
						l.logError(errChan, fmt.Errorf("sub err: %w", err))
						sub.Unsubscribe()
						client.Close()
						break connLoop
					case <-timer.C:
						l.logError(errChan, fmt.Errorf("heartbeat timeout (%v)", heartbeatInterval))
						sub.Unsubscribe()
						client.Close()
						break connLoop
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
		fmt.Printf("listener err: %v\n", err)
	}
}
