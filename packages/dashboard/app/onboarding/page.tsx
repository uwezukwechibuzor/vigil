"use client";
import { useState } from "react";
import { Button } from "../../components/ui/button";
import { Card, CardContent, CardHeader } from "../../components/ui/card";

const steps = ["Create project", "Add endpoints", "Copy API key", "First flight check"];

export default function OnboardingPage() {
  const [step, setStep] = useState(0);
  const [projectName, setProjectName] = useState("");
  const [apiKey] = useState("vgl_live_demo_key");

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <header>
        <h1 className="text-2xl font-semibold">Get started</h1>
        <p className="text-slate-500">Set up Vigil in under 5 minutes</p>
      </header>

      <nav aria-label="Onboarding steps">
        <ol className="flex gap-2">
          {steps.map((s, i) => (
            <li
              key={s}
              className={`flex-1 rounded-full py-1 text-center text-xs font-medium ${
                i < step
                  ? "bg-brand-500 text-white"
                  : i === step
                    ? "bg-brand-100 text-brand-700"
                    : "bg-slate-100 text-slate-400"
              }`}
            >
              {s}
            </li>
          ))}
        </ol>
      </nav>

      <Card>
        <CardHeader>
          <h2 className="font-medium">{steps[step]}</h2>
        </CardHeader>
        <CardContent>
          {step === 0 && (
            <div className="space-y-4">
              <label className="block">
                <span className="text-sm font-medium">Project name</span>
                <input
                  data-testid="project-name"
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  placeholder="My first project"
                />
              </label>
              <Button
                data-analytics-id="onboarding-create-project"
                onClick={() => setStep(1)}
                disabled={!projectName}
              >
                Create project
              </Button>
            </div>
          )}
          {step === 1 && (
            <div className="space-y-4">
              <p className="text-sm text-slate-600">Add an RPC endpoint to monitor.</p>
              <Button data-analytics-id="onboarding-add-endpoint" onClick={() => setStep(2)}>
                Add endpoint
              </Button>
            </div>
          )}
          {step === 2 && (
            <div className="space-y-4">
              <p className="text-sm text-slate-600">Your API key — copy it now, it will not be shown again.</p>
              <code data-testid="api-key" className="block rounded bg-slate-100 px-4 py-3 text-sm font-mono dark:bg-slate-800">
                {apiKey}
              </code>
              <Button data-analytics-id="onboarding-copy-key" onClick={() => setStep(3)}>
                I&apos;ve copied my key
              </Button>
            </div>
          )}
          {step === 3 && (
            <div className="space-y-4">
              <p className="text-sm text-slate-600">Run your first flight check:</p>
              <pre className="rounded bg-slate-100 px-4 py-3 text-xs dark:bg-slate-800">
                {`curl -X POST https://api.vigil.dev/v1/flight-checks \\
  -H "Authorization: Bearer ${apiKey}" \\
  -d '{"chain_id":8453,"from":"0x...","to":"0x...","data":"0x","value":"0x0"}'`}
              </pre>
              <Button
                data-analytics-id="onboarding-done"
                variant="default"
                onClick={() => window.location.assign("/")}
              >
                Go to dashboard
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
