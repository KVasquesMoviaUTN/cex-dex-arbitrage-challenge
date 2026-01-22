import { useEffect, useRef } from 'react';
import { useStore } from '../lib/store';
import { ArbitrageEvent } from '../lib/types';

const MOCK_MODE = false; // Set to true to test
const WS_URL = process.env.NEXT_PUBLIC_API_URL 
  ? `${process.env.NEXT_PUBLIC_API_URL.replace(/\/$/, '').replace('http', 'ws')}/ws`
  : 'ws://localhost:8080/ws';

export function useArbitrageSocket() {
  const socketRef = useRef<WebSocket | null>(null);
  const { setConnected, addEvent, updateLatency } = useStore();
  const mockIntervalRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    if (MOCK_MODE) {
      startMockMode();
      return () => stopMockMode();
    }

    connect();

    return () => {
      if (socketRef.current) {
        socketRef.current.close();
      }
    };
  }, []);

  const connect = () => {
    const ws = new WebSocket(WS_URL);
    socketRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      console.log('Connected to WebSocket');
    };

    ws.onclose = () => {
      setConnected(false);
      console.log('Disconnected. Reconnecting in 3s...');
      setTimeout(connect, 3000);
    };

    ws.onmessage = (msg) => {
      const now = Date.now();
      try {
        const event: ArbitrageEvent = JSON.parse(msg.data);
        // Calculate latency if timestamp is present
        if (event.timestamp) {
          const eventTime = new Date(event.timestamp).getTime();
          updateLatency(now - eventTime);
        }
        addEvent(event);
      } catch (e) {
        console.error('Failed to parse message', e);
      }
    };
  };

  const startMockMode = () => {
    console.log('Starting Mock Mode');
    setConnected(true);
    
    let currentBlock = 18000000;

    mockIntervalRef.current = setInterval(() => {
      currentBlock++;
      const isOpportunity = Math.random() > 0.3; // 70% chance of opportunity
      
      const event: ArbitrageEvent = {
        type: isOpportunity ? 'OPPORTUNITY' : 'HEARTBEAT',
        blockNumber: currentBlock,
        timestamp: new Date().toISOString(),
        data: isOpportunity ? {
          cexPrice: 3000 + Math.random() * 50,
          dexPrice: 3000 + Math.random() * 50,
          spreadPct: (Math.random() * 2) - 1, // -1% to +1%
          estimatedProfit: (Math.random() * 100) - 20, // -20 to +80
          gasCost: 5 + Math.random() * 5,
          symbol: 'ETH-USDC',
          direction: Math.random() > 0.5 ? 'CEX -> DEX' : 'DEX -> CEX'
        } : undefined
      };

      addEvent(event);
      updateLatency(Math.floor(Math.random() * 50) + 10);
    }, 2000); // New block every 2s (faster than real life for demo)
  };

  const stopMockMode = () => {
    if (mockIntervalRef.current) {
      clearInterval(mockIntervalRef.current);
    }
  };
}
