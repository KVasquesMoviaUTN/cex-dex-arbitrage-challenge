import { useStore } from '@/lib/store';
import { ArrowRight, TrendingUp, TrendingDown, Minus, Zap } from 'lucide-react';
import { clsx } from 'clsx';

export default function LiveFeed() {
	const events = useStore((state) => state.events);

	return (
		<div className="flex flex-col h-full bg-gradient-to-br from-slate-900 via-slate-900 to-slate-800 rounded-xl border border-slate-700/50 overflow-hidden shadow-2xl">
			{/* Header with gradient */}
			<div className="relative px-4 py-3 bg-gradient-to-r from-slate-800 to-slate-900 border-b border-slate-700/50">
				<div className="absolute inset-0 bg-gradient-to-r from-emerald-500/5 to-blue-500/5" />
				<div className="relative flex justify-between items-center">
					<div className="flex items-center gap-2">
						<Zap className="w-4 h-4 text-emerald-400" />
						<h2 className="text-sm font-bold text-white uppercase tracking-wider">Live Opportunities</h2>
					</div>
					<span className="text-xs text-slate-400 font-mono bg-slate-800/50 px-2 py-1 rounded">
						{events.length} events
					</span>
				</div>
			</div>

			{/* Feed Content */}
			<div className="flex-1 overflow-y-auto p-3 space-y-2 scrollbar-thin scrollbar-thumb-slate-700 scrollbar-track-transparent">
				{events.length === 0 && (
					<div className="flex flex-col items-center justify-center h-full text-slate-500">
						<Zap className="w-12 h-12 mb-3 opacity-20" />
						<p className="text-sm italic">Waiting for opportunities...</p>
					</div>
				)}

				{events.map((event, i) => {
					const isHighValue = (event.data?.estimatedProfit || 0) > 50;
					const netProfit = event.data?.estimatedProfit || 0;

					if (event.type === 'HEARTBEAT') {
						return (
							<div key={i} className="flex items-center gap-2 text-slate-600 text-xs py-1.5 px-3 bg-slate-800/30 rounded-lg border border-slate-800/50">
								<div className="w-1.5 h-1.5 rounded-full bg-slate-600 animate-pulse" />
								<span className="font-mono">[{new Date(event.timestamp).toLocaleTimeString()}]</span>
								<span className="uppercase tracking-wide">Heartbeat</span>
								<span className="text-slate-700">•</span>
								<span className="font-mono">Block #{event.blockNumber}</span>
							</div>
						);
					}

					return (
						<div
							key={i}
							className={clsx(
								"relative group rounded-xl p-3 border transition-all duration-300 hover:scale-[1.01]",
								isHighValue
									? "bg-gradient-to-br from-emerald-950/40 to-emerald-900/20 border-emerald-500/30 shadow-lg shadow-emerald-500/10"
									: "bg-slate-800/40 backdrop-blur-sm border-slate-700/50 hover:border-slate-600/50"
							)}
						>
							{/* Glow effect for high value */}
							{isHighValue && (
								<div className="absolute inset-0 bg-gradient-to-r from-emerald-500/10 to-transparent rounded-xl" />
							)}

							<div className="relative space-y-2">
								{/* Top Row: Time + Block + Symbol */}
								<div className="flex items-center justify-between">
									<div className="flex items-center gap-3">
										<span className="text-xs text-slate-500 font-mono">
											{new Date(event.timestamp).toLocaleTimeString()}
										</span>
										<span className="text-xs font-mono text-blue-400 bg-blue-500/10 px-2 py-0.5 rounded">
											#{event.blockNumber}
										</span>
									</div>
									<span className="text-sm font-bold text-yellow-400 tracking-wide">
										{event.data?.symbol}
									</span>
								</div>

								{/* Middle Row: Price Comparison */}
								<div className="flex items-center justify-between bg-slate-900/50 rounded-lg p-2">
									<div className="flex items-center gap-2">
										<div className="text-center">
											<div className="text-xs text-slate-500 mb-0.5">CEX</div>
											<div className="text-sm font-mono font-semibold text-slate-300">
												${event.data?.cexPrice.toFixed(2)}
											</div>
										</div>
										<ArrowRight size={14} className="text-slate-600 mx-1" />
										<div className="text-center">
											<div className="text-xs text-slate-500 mb-0.5">DEX</div>
											<div className="text-sm font-mono font-semibold text-slate-300">
												${event.data?.dexPrice.toFixed(2)}
											</div>
										</div>
									</div>

									{/* Spread Badge */}
									<div className={clsx(
										"px-2 py-1 rounded-lg text-xs font-mono font-bold",
										(event.data?.spreadPct || 0) > 0
											? "bg-emerald-500/20 text-emerald-400"
											: "bg-red-500/20 text-red-400"
									)}>
										{(event.data?.spreadPct || 0) > 0 ? '+' : ''}{event.data?.spreadPct.toFixed(3)}%
									</div>
								</div>

								{/* Bottom Row: Profit Breakdown */}
								<div className="grid grid-cols-3 gap-2 text-xs">
									<div className="bg-slate-900/50 rounded-lg p-2">
										<div className="text-slate-500 mb-0.5">Gross</div>
										<div className="font-mono font-semibold text-slate-300">
											${((event.data?.estimatedProfit || 0) + (event.data?.gasCost || 0)).toFixed(2)}
										</div>
									</div>
									<div className="bg-slate-900/50 rounded-lg p-2">
										<div className="text-slate-500 mb-0.5">Gas</div>
										<div className="font-mono font-semibold text-red-400">
											-${(event.data?.gasCost || 0).toFixed(2)}
										</div>
									</div>
									<div className={clsx("rounded-lg p-2", netProfit === 0 ? "bg-slate-900/50" : netProfit > 0 ? "bg-emerald-500/20 border border-emerald-500/30" : "bg-red-500/20 border border-red-500/30")}>
										<div className="text-slate-500 mb-0.5 flex items-center gap-1">
											Net
											{netProfit > 0 ? <TrendingUp className="w-3 h-3" /> : netProfit < 0 ? <TrendingDown className="w-3 h-3" /> : <Minus className="w-3 h-3" />}
										</div>
										<div className={clsx(
											"font-mono font-bold",
											netProfit === 0 ? "text-white" :
												netProfit > 0 ? "text-emerald-400" : "text-red-400"
										)}>
											${netProfit.toFixed(2)}
										</div>
									</div>
								</div>
							</div>
						</div>
					);
				})}
			</div>
		</div>
	);
}
