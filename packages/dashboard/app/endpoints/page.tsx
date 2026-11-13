export default function EndpointsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Endpoints</h1>
      <p className="text-slate-500">Monitor RPC, indexer and oracle endpoints for your projects.</p>
      <div data-analytics-id="endpoints-list" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        Connect your API key to see endpoints.
      </div>
    </div>
  );
}
