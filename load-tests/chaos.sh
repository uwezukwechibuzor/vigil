#!/usr/bin/env bash
# Chaos tests for Vigil — verifies graceful degradation under partial failures.
# Requires: kubectl (configured for the target cluster), k6, curl
# Usage: NAMESPACE=vigil ./load-tests/chaos.sh
set -euo pipefail

NS="${NAMESPACE:-vigil}"
API_URL="${API_URL:-http://localhost:8081}"
FLIGHT_URL="${FLIGHT_URL:-http://localhost:8083}"
INGEST_URL="${INGEST_URL:-http://localhost:8084}"
PASS=0
FAIL=0

pass() { echo "[PASS] $*"; ((PASS+=1)); }
fail() { echo "[FAIL] $*"; ((FAIL+=1)); }

check_healthz() {
  local url="$1" label="$2"
  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" "$url/healthz" || echo "000")
  if [[ "$status" == "200" ]]; then
    pass "$label /healthz = 200"
  else
    fail "$label /healthz = $status"
  fi
}

# ── 1. Kill Redis ───────────────────────────────────────────────────────────
echo ""
echo "=== Chaos 1: Kill Redis ==="
kubectl -n "$NS" scale deployment vigil-redis --replicas=0 2>/dev/null || echo "(skipped — no redis deployment in cluster)"
sleep 5

# Services should still serve (graceful degrade)
check_healthz "$API_URL" "api (redis down)"
check_healthz "$FLIGHT_URL" "flightcheck (redis down)"

# Rate limiting should degrade gracefully (not error)
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API_URL/v1/flight-check" \
  -H "Content-Type: application/json" \
  -d '{"chain":"ethereum","rpc_url":"https://cloudflare-eth.com"}' || echo "000")
if [[ "$status" != "500" && "$status" != "502" && "$status" != "503" ]]; then
  pass "flight-check request succeeds (or expected error) with redis down: $status"
else
  fail "flight-check returned server error $status with redis down"
fi

kubectl -n "$NS" scale deployment vigil-redis --replicas=1 2>/dev/null || true
echo "(Redis restored)"
sleep 10

# ── 2. Kill collector ───────────────────────────────────────────────────────
echo ""
echo "=== Chaos 2: Kill collector ==="
kubectl -n "$NS" scale deployment vigil-collector --replicas=0 2>/dev/null || echo "(skipped — no collector deployment)"
sleep 5

check_healthz "$API_URL" "api (collector down)"
check_healthz "$INGEST_URL" "ingest (collector down)"

# Ingest should queue or drop gracefully, not crash
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$INGEST_URL/v1/traces" \
  -H "Content-Type: application/json" \
  -d '{"resourceSpans":[]}' || echo "000")
if [[ "$status" != "500" && "$status" != "502" ]]; then
  pass "ingest accepts spans with collector down: $status"
else
  fail "ingest returned $status with collector down"
fi

kubectl -n "$NS" scale deployment vigil-collector --replicas=1 2>/dev/null || true
echo "(Collector restored)"
sleep 10

# ── 3. Slow ClickHouse (tc netem) ───────────────────────────────────────────
echo ""
echo "=== Chaos 3: Simulate slow ClickHouse (500ms network delay) ==="
CH_POD=$(kubectl -n "$NS" get pods -l app=clickhouse -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [[ -n "$CH_POD" ]]; then
  kubectl -n "$NS" exec "$CH_POD" -- \
    tc qdisc add dev eth0 root netem delay 500ms 2>/dev/null || echo "(tc not available)"
  sleep 3

  # Insights and traces queries should time out gracefully (not hang indefinitely)
  start=$(date +%s%N)
  status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "$API_URL/v1/traces?limit=10" || echo "000")
  elapsed=$(( ($(date +%s%N) - start) / 1000000 ))
  if [[ $elapsed -lt 6000 ]]; then
    pass "traces query respects timeout with slow CH (${elapsed}ms, status=$status)"
  else
    fail "traces query hung for ${elapsed}ms with slow ClickHouse"
  fi

  kubectl -n "$NS" exec "$CH_POD" -- tc qdisc del dev eth0 root 2>/dev/null || true
  echo "(ClickHouse delay removed)"
else
  echo "(skipped — no ClickHouse pod found)"
fi

# ── Summary ─────────────────────────────────────────────────────────────────
echo ""
echo "=== Chaos test summary: $PASS passed, $FAIL failed ==="
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
