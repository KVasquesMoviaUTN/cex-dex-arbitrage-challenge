package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/domain"
	"github.com/KVasquesMoviaUTN/arbitrage-bot-go/internal/core/ports"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/time/rate"
)

type Listener struct {
	clientURL string
	lastBlock *big.Int
	limiter   *rate.Limiter
}

func NewListener(clientURL string) ports.BlockchainListener {
	return &Listener{
		clientURL: clientURL,
		limiter:   rate.NewLimiter(rate.Limit(20), 5), // 20 req/s, burst 5
	}
}

func (l *Listener) SubscribeNewHeads(ctx context.Context) (<-chan *domain.Block, <-chan error, error) {
	out := make(chan *domain.Block)
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
					continue
				}

				// Backfill missing blocks
				if l.lastBlock != nil {
					head, err := client.HeaderByNumber(ctx, nil)
					if err == nil && head.Number.Cmp(l.lastBlock) > 0 {
						start := new(big.Int).Add(l.lastBlock, big.NewInt(1))
						end := head.Number
						
						// Limit backfill to 50 blocks
						if new(big.Int).Sub(end, start).Cmp(big.NewInt(50)) > 0 {
							start = new(big.Int).Sub(end, big.NewInt(50))
						}

						for i := new(big.Int).Set(start); i.Cmp(end) <= 0; i.Add(i, big.NewInt(1)) {
							if err := l.limiter.Wait(ctx); err != nil {
								l.logError(errChan, fmt.Errorf("rate limiter wait failed: %w", err))
								break
							}
							block, err := client.BlockByNumber(ctx, i)
							if err != nil {
								l.logError(errChan, fmt.Errorf("backfill failed for block %s: %w", i, err))
								continue
							}
							out <- &domain.Block{
								Number:    block.Number(),
								Timestamp: time.Unix(int64(block.Time()), 0),
							}
							l.lastBlock = block.Number()
						}
					}
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
						block := &domain.Block{
							Number:    header.Number,
							Timestamp: time.Unix(int64(header.Time), 0),
						}
						l.lastBlock = header.Number
						select {
						case out <- block:
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
