export default function FlightChecksPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Flight Checks</h1>
      <p className="text-slate-500">Pre-transaction safety checks run before signing.</p>
      <div data-analytics-id="flight-checks-list" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        No flight checks recorded yet.
      </div>
    </div>
  );
}
