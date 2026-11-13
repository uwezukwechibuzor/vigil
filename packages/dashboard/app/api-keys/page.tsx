export default function ApiKeysPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">API Keys</h1>
      <p className="text-slate-500">Manage API keys for SDK and direct API access.</p>
      <div data-analytics-id="api-keys-list" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        No API keys created.
      </div>
    </div>
  );
}
