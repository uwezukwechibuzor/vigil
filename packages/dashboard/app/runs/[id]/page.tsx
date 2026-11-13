export default async function RunDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  const spans = [
    { id: "span-1", name: "agent.run", service: "agent", duration: 2100, type: "root" },
    { id: "span-2", name: "gen_ai.chat", service: "openai", duration: 850, type: "llm", model: "gpt-4o", tokens: 1240 },
    { id: "span-3", name: "vigil.flight_check", service: "vigil", duration: 180, type: "tool" },
    { id: "span-4", name: "eth_sendRawTransaction", service: "web3", duration: 320, type: "tx", txHash: "0xabc...def" },
  ];

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold">Run {id}</h1>
        <p className="text-slate-500">Agent run trace waterfall</p>
      </header>
      <div data-analytics-id="run-trace-waterfall" className="space-y-1">
        {spans.map((s) => (
          <div key={s.id} className="flex items-center gap-4 rounded-lg border border-slate-200 px-4 py-3 text-sm dark:border-slate-700">
            <span className="w-32 font-mono text-xs text-slate-500">{s.id}</span>
            <span className="flex-1 font-medium">{s.name}</span>
            <span className="text-slate-500">{s.service}</span>
            {"model" in s && s.model && (
              <span className="text-xs text-blue-600">{s.model} ({s.tokens} tok)</span>
            )}
            {"txHash" in s && s.txHash && (
              <span className="font-mono text-xs text-green-600">{s.txHash}</span>
            )}
            <span className="text-slate-400">{s.duration}ms</span>
          </div>
        ))}
      </div>
    </div>
  );
}
