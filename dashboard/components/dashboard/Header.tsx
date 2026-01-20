import { useStore } from '@/lib/store';
import { Activity, Wifi, WifiOff } from 'lucide-react';

export default function Header() {
	const { isConnected, lastBlock, latency } = useStore();

	return (
		<header className="flex items-center justify-between px-6 py-4 bg-slate-900 border-b border-slate-800">
			<div className="flex items-center gap-4">
				<h1 className="text-xl font-bold text-white tracking-wider">
					<span className="text-emerald-500">CEX-DEX</span> Arbitrage Bot <span className="text-slate-500 text-sm font-normal">by Kalil</span>
				</h1>
			</div>

			<div className="flex items-center gap-8">
				{/* Block Height */}
				<div className="flex flex-col items-end">
					<span className="text-xs text-slate-400 uppercase tracking-wider">Block Height</span>
					<span className="text-2xl font-mono font-bold text-blue-400">
						#{lastBlock.toLocaleString()}
					</span>
				</div>

				{/* Latency */}
				<div className="flex flex-col items-end">
					<span className="text-xs text-slate-400 uppercase tracking-wider">Latency</span>
					<div className="flex items-center gap-2">
						<Activity size={16} className="text-slate-500" />
						<span className={`font-mono font-bold ${latency > 200 ? 'text-yellow-500' : 'text-emerald-500'}`}>
							{latency}ms
						</span>
					</div>
				</div>

				{/* Connection Status */}
				<div className={`flex items-center gap-2 px-3 py-1 rounded-full border ${isConnected
						? 'bg-emerald-500/20 border-emerald-500/50'
						: 'bg-red-500/20 border-red-500/50'
					}`}>
					{isConnected ? (
						<>
							<div className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
							<span className="text-sm text-emerald-500 font-medium">Online</span>
						</>
					) : (
						<>
							<div className="w-2 h-2 rounded-full bg-red-500" />
							<span className="text-sm text-red-500 font-medium">Offline</span>
						</>
					)}
				</div>
			</div>
		</header>
	);
}
