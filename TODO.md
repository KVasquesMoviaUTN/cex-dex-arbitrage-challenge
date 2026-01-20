# Technical Debt & Future Improvements

## High Priority (Reliability)
- [x] **Circuit Breaker**: Implement a circuit breaker for the Binance API to prevent cascading failures and IP bans (HTTP 429).
- [ ] **Rate Limiting**: Add a rate limiter to the `BinanceAdapter` to respect API limits.
- [ ] **Robust Reconnection**: Update `BlockchainListener` to track `lastProcessedBlock` and backfill missing blocks upon reconnection.

## Medium Priority (Accuracy)
- [ ] **Dynamic Gas Pricing**: Replace the hardcoded 30 Gwei gas price with a dynamic fetch from the Ethereum node (`eth_gasPrice` or `eth_maxFeePerGas`).
- [ ] **Two-Way Arbitrage**: Implement `DEX Buy -> CEX Sell` logic. Currently, we only check `CEX Buy -> DEX Sell`.
- [ ] **Accurate Gas Estimation**: Simulate the full transaction (including transfers) for better profit calculation, rather than just the swap gas.

## Low Priority (Features)
- [ ] **Execution Logic**: Implement the actual trade execution (signing transactions, sending orders).
- [ ] **Integration Tests**: Add tests running against a local Anvil/Hardhat fork to verify contract interactions.
