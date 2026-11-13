export default function UsagePage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Usage &amp; Billing</h1>
      <p className="text-slate-500">Track API calls, flight checks, and spans ingested.</p>
      <div data-analytics-id="usage-chart" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        Usage data will appear here.
      </div>
    </div>
  );
}
