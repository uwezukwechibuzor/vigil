import Link from "next/link";

const mockRuns = [
  { id: "run-demo-123", agent: "defi-agent", status: "completed", duration: "2.1s", model: "gpt-4o", spans: 4 },
  { id: "run-demo-456", agent: "arb-bot", status: "failed", duration: "0.4s", model: "claude-haiku-4-5-20251001", spans: 2 },
];

export default function RunsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Agent Runs</h1>
      <p className="text-slate-500">End-to-end traces from LLM reasoning to onchain result.</p>
      <div data-analytics-id="runs-list" className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 dark:bg-slate-800">
            <tr>
              <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Run ID</th>
              <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Agent</th>
              <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Status</th>
              <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Duration</th>
              <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Spans</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
            {mockRuns.map((r) => (
              <tr key={r.id} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                <td className="px-4 py-3">
                  <Link href={`/runs/${r.id}`} className="font-mono text-xs text-brand-600 hover:underline">{r.id}</Link>
                </td>
                <td className="px-4 py-3">{r.agent}</td>
                <td className="px-4 py-3">{r.status}</td>
                <td className="px-4 py-3">{r.duration}</td>
                <td className="px-4 py-3">{r.spans}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
