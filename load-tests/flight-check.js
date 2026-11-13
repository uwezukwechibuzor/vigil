// k6 load test: flight check endpoint at 200 rps
// Run: k6 run --env BASE_URL=http://localhost:8083 load-tests/flight-check.js
import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const errorRate = new Rate("errors");
const p95Latency = new Trend("p95_latency", true);

export const options = {
  scenarios: {
    ramp_up: {
      executor: "ramping-arrival-rate",
      startRate: 10,
      timeUnit: "1s",
      preAllocatedVUs: 50,
      maxVUs: 200,
      stages: [
        { duration: "30s", target: 50 },
        { duration: "2m", target: 200 },
        { duration: "1m", target: 200 },
        { duration: "30s", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<1500"], // SLO: p95 < 1.5s
    errors: ["rate<0.01"],            // <1% error rate
    http_req_failed: ["rate<0.01"],
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8083";

const payload = JSON.stringify({
  chain: "ethereum",
  rpc_url: "https://mainnet.infura.io/v3/test",
  tx: {
    from: "0x742d35Cc6634C0532925a3b8D4C9b51A",
    to: "0xA0b86a4D92c8c5D2F87E5b57f7c6B9d3e0c4f8A",
    value: "0xDE0B6B3A7640000",
    data: "0x",
  },
});

export default function () {
  const res = http.post(`${BASE_URL}/v1/flight-check`, payload, {
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer test-token",
    },
    timeout: "5s",
  });

  const ok = check(res, {
    "status 200": (r) => r.status === 200,
    "has risk_score": (r) => {
      try {
        const body = JSON.parse(r.body);
        return typeof body.risk_score === "number";
      } catch {
        return false;
      }
    },
    "latency < 2s": (r) => r.timings.duration < 2000,
  });

  errorRate.add(!ok);
  p95Latency.add(res.timings.duration);

  sleep(0.001); // minimal think time for arrival-rate scenario
}

export function handleSummary(data) {
  return {
    "docs/benchmarks-flight-check-raw.json": JSON.stringify(data, null, 2),
  };
}
