export default function TeamSettingsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Team Settings</h1>
      <p className="text-slate-500">Manage team members, roles, and organization settings.</p>
      <div data-analytics-id="team-members" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        No team members added.
      </div>
    </div>
  );
}
