import { useStore } from '@/lib/store';
import { Activity, TrendingUp, Zap, Radio } from 'lucide-react';

export default function Header() {
	const { isConnected, lastBlock, latency, events } = useStore();

	// Calculate stats from recent events
	const recentEvents = events.slice(-20);
	const profitableOps = recentEvents.filter(e => e.data && e.data.estimatedProfit > 0).length;
	const avgSpread = recentEvents.length > 0
		? recentEvents.reduce((acc, e) => acc + (e.data?.spreadPct || 0), 0) / recentEvents.length
		: 0;

	return (
		<header className="relative bg-gradient-to-r from-slate-900 via-slate-900 to-slate-800 border-b border-slate-700/50 backdrop-blur-xl">
			{/* Animated gradient overlay */}
			<div className="absolute inset-0 bg-gradient-to-r from-emerald-500/5 via-transparent to-blue-500/5 animate-pulse" />

			<div className="relative px-6 py-4">
				{/* Top Row: Branding + Status */}
				<div className="flex items-center justify-between mb-4">
					<div className="flex items-center gap-3">
						<div className="relative">
							<div className="absolute inset-0 bg-emerald-500/20 blur-xl rounded-full" />
							<div className="relative w-10 h-10 bg-gradient-to-br from-emerald-400 to-emerald-600 rounded-lg flex items-center justify-center shadow-lg shadow-emerald-500/50">
								<TrendingUp className="w-6 h-6 text-white" />
							</div>
						</div>
						<div>
							<h1 className="text-2xl font-bold text-white">
								CEX-DEX Arbitrage
							</h1>
							<p className="text-xs text-slate-400 font-medium">Real-time ETH/USDC Opportunities</p>
						</div>
					</div>

					{/* Connection Status - Premium */}
					<div className={`flex items-center gap-2 px-4 py-2 rounded-full transition-all duration-300 ${isConnected
							? 'bg-emerald-500 shadow-lg shadow-emerald-500/50'
							: 'bg-red-500 shadow-lg shadow-red-500/50'
						}`}>
						<Radio className="w-4 h-4 text-white animate-pulse" />
						<span className="text-sm font-semibold text-white">
							{isConnected ? 'LIVE' : 'OFFLINE'}
						</span>
					</div>
				</div>

				{/* Bottom Row: Metrics Grid */}
				<div className="grid grid-cols-2 md:grid-cols-4 gap-4">
					{/* Block Height */}
					<div className="bg-slate-800/40 backdrop-blur-sm rounded-lg p-3 border border-slate-700/50 hover:border-blue-500/50 transition-all duration-300 group">
						<div className="flex items-center gap-2 mb-1">
							<Zap className="w-3.5 h-3.5 text-blue-400 group-hover:text-blue-300 transition-colors" />
							<span className="text-xs text-slate-400 uppercase tracking-wider font-medium">Block</span>
						</div>
						<div className="text-xl font-bold text-blue-400 font-mono">
							#{lastBlock.toLocaleString()}
						</div>
					</div>

					{/* Latency */}
					<div className="bg-slate-800/40 backdrop-blur-sm rounded-lg p-3 border border-slate-700/50 hover:border-emerald-500/50 transition-all duration-300 group">
						<div className="flex items-center gap-2 mb-1">
							<Activity className="w-3.5 h-3.5 text-emerald-400 group-hover:text-emerald-300 transition-colors" />
							<span className="text-xs text-slate-400 uppercase tracking-wider font-medium">Latency</span>
						</div>
						<div className={`text-xl font-bold font-mono ${latency > 200 ? 'text-yellow-400' : 'text-emerald-400'}`}>
							{latency}ms
						</div>
					</div>

					{/* Avg Spread */}
					<div className="bg-slate-800/40 backdrop-blur-sm rounded-lg p-3 border border-slate-700/50 hover:border-purple-500/50 transition-all duration-300 group">
						<div className="flex items-center gap-2 mb-1">
							<TrendingUp className="w-3.5 h-3.5 text-purple-400 group-hover:text-purple-300 transition-colors" />
							<span className="text-xs text-slate-400 uppercase tracking-wider font-medium">Avg Spread</span>
						</div>
						<div className={`text-xl font-bold font-mono ${avgSpread > 0 ? 'text-emerald-400' : 'text-red-400'}`}>
							{avgSpread.toFixed(3)}%
						</div>
					</div>

					{/* Profitable Ops */}
					<div className="bg-slate-800/40 backdrop-blur-sm rounded-lg p-3 border border-slate-700/50 hover:border-amber-500/50 transition-all duration-300 group">
						<div className="flex items-center gap-2 mb-1">
							<TrendingUp className="w-3.5 h-3.5 text-amber-400 group-hover:text-amber-300 transition-colors" />
							<span className="text-xs text-slate-400 uppercase tracking-wider font-medium">Profitable</span>
						</div>
						<div className="text-xl font-bold text-amber-400 font-mono">
							{profitableOps}/{recentEvents.length}
						</div>
					</div>
				</div>
			</div>
		</header>
	);
}
