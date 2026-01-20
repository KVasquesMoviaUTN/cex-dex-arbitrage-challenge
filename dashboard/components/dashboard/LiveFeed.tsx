import { useStore } from '@/lib/store';
import { ArrowRight } from 'lucide-react';
import { clsx } from 'clsx';

export default function LiveFeed() {
	const events = useStore((state) => state.events);

	return (
		<div className="flex flex-col h-full bg-slate-950 rounded-lg border border-slate-800 overflow-hidden">
			<div className="px-4 py-2 bg-slate-900 border-b border-slate-800 flex justify-between items-center">
				<h2 className="text-sm font-semibold text-slate-300 uppercase tracking-wider">Live Feed</h2>
				<span className="text-xs text-slate-500 font-mono">buffer: 100</span>
			</div>

			<div className="flex-1 overflow-y-auto p-2 space-y-1 font-mono text-sm scrollbar-thin scrollbar-thumb-slate-700 scrollbar-track-transparent">
				{events.length === 0 && (
					<div className="text-slate-600 text-center py-10 italic">Waiting for events...</div>
				)}

				{events.map((event, i) => {
					const isHighValue = (event.data?.estimatedProfit || 0) > 50;

					if (event.type === 'HEARTBEAT') {
						return (
							<div key={i} className="flex items-center gap-2 text-slate-600 opacity-50 text-xs py-1">
								<span>[{new Date(event.timestamp).toLocaleTimeString()}]</span>
								<span>HEARTBEAT</span>
								<span>Block: {event.blockNumber}</span>
							</div>
						);
					}

					return (
						<div
							key={i}
							className={clsx(
								"grid grid-cols-12 gap-2 p-2 rounded border-l-2 transition-colors",
								isHighValue
									? "bg-emerald-950/30 border-emerald-500 text-emerald-200"
									: "bg-slate-900/50 border-slate-700 text-slate-300 hover:bg-slate-800"
							)}
						>
							<div className="col-span-2 text-slate-500 text-xs flex items-center">
								{new Date(event.timestamp).toLocaleTimeString()}
							</div>

							<div className="col-span-1 font-bold text-blue-400">
								#{event.blockNumber}
							</div>

							<div className="col-span-2 text-yellow-500 font-bold">
								{event.data?.symbol}
							</div>

							<div className="col-span-4 flex flex-col justify-center">
								<div className="flex items-center gap-2">
									<span className="text-slate-400">{event.data?.cexPrice.toFixed(2)}</span>
									<ArrowRight size={12} className="text-slate-600" />
									<span className="text-slate-400">{event.data?.dexPrice.toFixed(2)}</span>
								</div>
								<span className={clsx("text-xs font-mono", (event.data?.spreadPct || 0) > 0 ? "text-emerald-500" : "text-red-500")}>
									{event.data?.spreadPct.toFixed(3)}%
								</span>
							</div>

							<div className={clsx(
								"col-span-3 text-right font-bold",
								(event.data?.estimatedProfit || 0) > 0 ? "text-emerald-400" : "text-red-400"
							)}>
								<div className="text-xs text-slate-500 font-normal">
									Gross: ${((event.data?.estimatedProfit || 0) + (event.data?.gasCost || 0)).toFixed(2)}
								</div>
								<div className="text-xs text-slate-500 font-normal">
									Gas: -${(event.data?.gasCost || 0).toFixed(2)}
								</div>
								<div>
									Net: ${(event.data?.estimatedProfit || 0).toFixed(2)}
								</div>
							</div>
						</div>
					);
				})}
			</div>
		</div>
	);
}
