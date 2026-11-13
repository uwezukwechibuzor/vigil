export default function OraclesPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Oracles</h1>
      <p className="text-slate-500">Track price feed staleness and deviation for oracle endpoints.</p>
      <div data-analytics-id="oracles-list" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        No oracles configured yet.
      </div>
    </div>
  );
}
