export default function IncidentsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Incidents</h1>
      <p className="text-slate-500">Correlated anomalies with AI root-cause summaries.</p>
      <div data-analytics-id="incidents-list" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        No incidents detected.
      </div>
    </div>
  );
}
