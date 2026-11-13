// k6 load test: OTLP ingest at 5,000 spans/s
// Run: k6 run --env BASE_URL=http://localhost:8084 load-tests/ingest.js
import http from "k6/http";
import { check } from "k6";
import { Rate } from "k6/metrics";
import encoding from "k6/encoding";

const errorRate = new Rate("errors");

export const options = {
  scenarios: {
    sustained: {
      executor: "constant-arrival-rate",
      rate: 5000,
      timeUnit: "1s",
      duration: "2m",
      preAllocatedVUs: 100,
      maxVUs: 500,
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    errors: ["rate<0.005"],
    http_req_failed: ["rate<0.005"],
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8084";

// Minimal valid OTLP/JSON payload with 1 span per request
function makeOTLPPayload(i) {
  return JSON.stringify({
    resourceSpans: [
      {
        resource: {
          attributes: [
            { key: "service.name", value: { stringValue: `load-test-${i % 10}` } },
          ],
        },
        scopeSpans: [
          {
            spans: [
              {
                traceId: encoding.b64encode(`trace${i}`.padEnd(16, "0").slice(0, 16)),
                spanId: encoding.b64encode(`span${i}`.padEnd(8, "0").slice(0, 8)),
                name: "agent.run",
                kind: 1,
                startTimeUnixNano: `${Date.now() - 100}000000`,
                endTimeUnixNano: `${Date.now()}000000`,
                attributes: [
                  { key: "vigil.agent.id", value: { stringValue: `agent-${i % 50}` } },
                  { key: "gen_ai.system", value: { stringValue: "anthropic" } },
                ],
                status: { code: 1 },
              },
            ],
          },
        ],
      },
    ],
  });
}

let counter = 0;

export default function () {
  const i = counter++;
  const res = http.post(`${BASE_URL}/v1/traces`, makeOTLPPayload(i), {
    headers: { "Content-Type": "application/json" },
    timeout: "2s",
  });

  const ok = check(res, {
    "status 200": (r) => r.status === 200,
  });

  errorRate.add(!ok);
}

export function handleSummary(data) {
  return {
    "docs/benchmarks-ingest-raw.json": JSON.stringify(data, null, 2),
  };
}
