import { Card, CardHeader, CardContent } from "../components/ui/card";
import { Badge } from "../components/ui/badge";

const chains = [
  { id: "1", name: "Ethereum", status: "healthy", lag: 0, fee: "normal" },
  { id: "8453", name: "Base", status: "healthy", lag: 1, fee: "low" },
  { id: "42161", name: "Arbitrum", status: "degraded", lag: 3, fee: "high" },
];

const verdictBadge = (s: string): "success" | "warning" | "danger" =>
  s === "healthy" ? "success" : s === "degraded" ? "warning" : "danger";

export default function OverviewPage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold">Overview</h1>
        <p className="text-sm text-slate-500">Chain and endpoint health across all monitored networks</p>
      </header>

      <section aria-label="Chain health">
        <h2 className="mb-3 text-lg font-medium">Chains</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {chains.map((c) => (
            <Card key={c.id}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <span className="font-medium">{c.name}</span>
                  <Badge variant={verdictBadge(c.status)}>{c.status}</Badge>
                </div>
              </CardHeader>
              <CardContent>
                <dl className="space-y-1 text-sm">
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Head lag</dt>
                    <dd>{c.lag} blocks</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Fee level</dt>
                    <dd>{c.fee}</dd>
                  </div>
                </dl>
              </CardContent>
            </Card>
          ))}
        </div>
      </section>
    </div>
  );
}
