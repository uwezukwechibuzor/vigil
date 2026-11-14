"use client";

import { useEffect, useState } from "react";
import { Card, CardHeader, CardContent } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { apiFetch, getMe, type Chain, type ChainHealth, type Me } from "../lib/api";

type ChainWithHealth = Chain & { health?: ChainHealth };

const statusVariant = (s: string): "success" | "warning" | "danger" =>
  s === "healthy" ? "success" : s === "degraded" ? "warning" : "danger";

export default function OverviewPage() {
  const [chains, setChains] = useState<ChainWithHealth[]>([]);
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const [meData, chainList] = await Promise.all([
          getMe(),
          apiFetch<{ data: Chain[] }>("/v1/chains"),
        ]);
        setMe(meData);
        const withHealth = await Promise.all(
          chainList.data.map(async (c) => {
            try {
              const h = await apiFetch<ChainHealth>(`/v1/chains/${c.chain_id}/health`);
              return { ...c, health: h };
            } catch {
              return c;
            }
          })
        );
        setChains(withHealth);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold">Overview</h1>
        <p className="text-sm text-slate-500">Chain and endpoint health across all monitored networks</p>
      </header>

      {me && (
        <p className="text-xs text-slate-400">Org: {me.org_id}{me.project_id ? ` · Project: ${me.project_id}` : ""}</p>
      )}

      {loading && <p className="text-sm text-slate-500">Loading chains…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && (
        <section aria-label="Chain health">
          <h2 className="mb-3 text-lg font-medium">Chains</h2>
          {chains.length === 0 ? (
            <p className="text-sm text-slate-400">No chains configured.</p>
          ) : (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {chains.map((c) => {
                const status = c.health?.status ?? "unknown";
                return (
                  <Card key={c.id}>
                    <CardHeader>
                      <div className="flex items-center justify-between">
                        <span className="font-medium">{c.name}</span>
                        <Badge variant={statusVariant(status)}>{status}</Badge>
                      </div>
                    </CardHeader>
                    <CardContent>
                      <dl className="space-y-1 text-sm">
                        <div className="flex justify-between">
                          <dt className="text-slate-500">Head lag</dt>
                          <dd>{c.health?.head_lag_blocks ?? "—"} blocks</dd>
                        </div>
                        <div className="flex justify-between">
                          <dt className="text-slate-500">Fee level</dt>
                          <dd>{c.health?.fee_level ?? "—"}</dd>
                        </div>
                        <div className="flex justify-between">
                          <dt className="text-slate-500">Reorgs 24h</dt>
                          <dd>{c.health?.reorgs_24h ?? "—"}</dd>
                        </div>
                      </dl>
                    </CardContent>
                  </Card>
                );
              })}
            </div>
          )}
        </section>
      )}
    </div>
  );
}
