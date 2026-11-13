"use client";
import { useState } from "react";
import { Button } from "../../../components/ui/button";
import { Card, CardContent, CardHeader } from "../../../components/ui/card";

export default function NewAlertRulePage() {
  const [name, setName] = useState("");
  const [severity, setSeverity] = useState("medium");
  const [saved, setSaved] = useState(false);

  return (
    <div className="mx-auto max-w-xl space-y-6">
      <h1 className="text-2xl font-semibold">New alert rule</h1>
      {saved ? (
        <div data-testid="rule-saved" className="rounded-lg bg-green-50 p-4 text-green-800">
          Alert rule &ldquo;{name}&rdquo; created successfully.
        </div>
      ) : (
        <Card>
          <CardHeader>
            <h2 className="font-medium">Rule details</h2>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <label className="block">
                <span className="text-sm font-medium">Rule name</span>
                <input
                  data-testid="rule-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  placeholder="RPC latency spike"
                />
              </label>
              <label className="block">
                <span className="text-sm font-medium">Severity</span>
                <select
                  data-testid="rule-severity"
                  value={severity}
                  onChange={(e) => setSeverity(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 text-sm"
                >
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                  <option value="critical">Critical</option>
                </select>
              </label>
              <Button
                data-testid="save-rule"
                data-analytics-id="alert-rule-save"
                onClick={() => setSaved(true)}
                disabled={!name}
              >
                Save rule
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
