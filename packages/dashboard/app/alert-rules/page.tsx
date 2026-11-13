import Link from "next/link";
import { Button } from "../../components/ui/button";

export default function AlertRulesPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Alert Rules</h1>
        <Link href="/alert-rules/new">
          <Button size="sm">New rule</Button>
        </Link>
      </div>
      <p className="text-slate-500">Trigger notifications when thresholds are exceeded.</p>
      <div data-analytics-id="alert-rules-list" className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
        No alert rules configured.
      </div>
    </div>
  );
}
