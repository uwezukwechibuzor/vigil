"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { apiFetch, getMe, type AlertRule } from "../../lib/api";

const severityVariant = (s: string): "danger" | "warning" | "success" =>
  s === "critical" ? "danger" : s === "high" ? "warning" : "success";

export default function AlertRulesPage() {
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const me = await getMe();
        if (!me.project_id) {
          setLoading(false);
          return;
        }
        const data = await apiFetch<{ data: AlertRule[] }>(`/v1/alert-rules?project_id=${me.project_id}`);
        setRules(data.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Alert Rules</h1>
        <Link href="/alert-rules/new">
          <Button size="sm">New rule</Button>
        </Link>
      </div>
      <p className="text-slate-500">Trigger notifications when thresholds are exceeded.</p>

      {loading && <p className="text-sm text-slate-500">Loading alert rules…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && rules.length === 0 && (
        <div className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
          No alert rules configured.
        </div>
      )}

      {!loading && !error && rules.length > 0 && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-800">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Name</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Severity</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Status</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {rules.map((r) => (
                <tr key={r.id} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-3 font-medium">{r.name}</td>
                  <td className="px-4 py-3">
                    <Badge variant={severityVariant(r.severity)}>{r.severity}</Badge>
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant={r.enabled ? "success" : "warning"}>
                      {r.enabled ? "active" : "disabled"}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-slate-500">{new Date(r.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
