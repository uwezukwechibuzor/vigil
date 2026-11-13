export default function PoliciesPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Policies</h1>
      <p className="text-slate-500">Configure flight-check policies for your agents.</p>
      <div data-analytics-id="policies-list" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        No policies defined.
      </div>
    </div>
  );
}
