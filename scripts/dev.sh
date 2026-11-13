#!/bin/bash
# dev.sh — start the full Vigil stack locally without Docker
#
# Starts Postgres, Redis, ClickHouse, and Anvil natively, then builds and
# launches all six Go services. All data lives under tmp/ so the checkout
# stays clean. Ctrl-C stops services and (by default) infra too.
#
# Prerequisites (install once via Homebrew on macOS):
#   brew install postgresql@16 redis clickhouse
#   foundryup                    # for Anvil (optional — EVM dev chain)
#
# Usage:
#   ./scripts/dev.sh                 start everything (first run seeds demo data)
#   ./scripts/dev.sh --no-seed       skip demo data seeding
#   ./scripts/dev.sh --no-build      skip go build (use existing bin/)
#   ./scripts/dev.sh --keep-infra    leave Postgres/Redis/ClickHouse running on Ctrl-C
#   ./scripts/dev.sh --infra-only    start infra + migrate, skip Go services
#   ./scripts/dev.sh --stop          stop all services and infra we started
#   ./scripts/dev.sh --reset         wipe tmp/data/ and start fresh
#   ./scripts/dev.sh --logs          tail logs of a running session

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# Homebrew versioned postgres installs only symlink pg_ctl into /opt/homebrew/bin,
# not initdb/psql. Detect the full bin dir and prepend it to PATH.
if command -v pg_config >/dev/null 2>&1; then
  export PATH="$(pg_config --bindir):$PATH"
elif ls -d /opt/homebrew/opt/postgresql*/bin 2>/dev/null | grep -q .; then
  PG_BIN=$(ls -d /opt/homebrew/opt/postgresql*/bin 2>/dev/null | sort -t@ -k2 -V | tail -1)
  export PATH="$PG_BIN:$PATH"
fi

TMP_DIR="$REPO_ROOT/tmp"
DATA_DIR="$TMP_DIR/data"
LOG_DIR="$TMP_DIR/logs"
PID_DIR="$TMP_DIR/pids"
SEED_MARKER="$TMP_DIR/.seeded"
BIN_DIR="$REPO_ROOT/bin"

# Infra ports
PG_PORT=5432
REDIS_PORT=6379
CH_HTTP_PORT=8123
CH_TCP_PORT=9000
ANVIL_PORT=8545

# Ordered list of Go services
SERVICES="api collector flightcheck ingest insights alerts"

# Service colors (indexed, bash 3.2 compatible)
SVC_COLORS=('\033[0;34m' '\033[0;35m' '\033[0;36m' '\033[0;33m' '\033[0;32m' '\033[0;31m')

# Track which infra WE started (so we stop only what we own)
STARTED_PG=0
STARTED_REDIS=0
STARTED_CH=0
STARTED_ANVIL=0

# -- Flags ----------------------------------------------------------------------
OPT_STOP=0
OPT_NO_SEED=0
OPT_NO_BUILD=0
OPT_KEEP_INFRA=0
OPT_INFRA_ONLY=0
OPT_RESET=0
OPT_LOGS=0

for arg in "$@"; do
  case "$arg" in
    --stop)        OPT_STOP=1 ;;
    --no-seed)     OPT_NO_SEED=1 ;;
    --no-build)    OPT_NO_BUILD=1 ;;
    --keep-infra)  OPT_KEEP_INFRA=1 ;;
    --infra-only)  OPT_INFRA_ONLY=1 ;;
    --reset)       OPT_RESET=1 ;;
    --logs)        OPT_LOGS=1 ;;
    *) echo "Unknown flag: $arg" >&2; exit 1 ;;
  esac
done

# -- Colors ---------------------------------------------------------------------
BOLD='\033[1m'
RESET='\033[0m'
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'

log()  { printf "${BOLD}[dev]${RESET} %s\n" "$*"; }
ok()   { printf "${GREEN}[dev]${RESET} ✓ %s\n" "$*"; }
info() { printf "${CYAN}[dev]${RESET}   %s\n" "$*"; }
warn() { printf "${YELLOW}[dev]${RESET} ⚠ %s\n" "$*"; }
die()  { printf "${RED}[dev]${RESET} ✗ %s\n" "$*" >&2; exit 1; }

# -- Port lookup (replaces associative array) -----------------------------------
svc_port() {
  case "$1" in
    api)         echo 8080 ;;
    collector)   echo 8081 ;;
    flightcheck) echo 8082 ;;
    ingest)      echo 8083 ;;
    insights)    echo 8084 ;;
    alerts)      echo 8085 ;;
    *) die "Unknown service: $1" ;;
  esac
}

# -- Helpers --------------------------------------------------------------------
port_open() { nc -z 127.0.0.1 "$1" 2>/dev/null; }

wait_port() {
  local port=$1 label=$2 max=${3:-30} i=0
  while ! port_open "$port"; do
    i=$((i + 1))
    [ "$i" -ge "$max" ] && die "Timed out waiting for $label on port $port"
    printf '\r  waiting for %s...  ' "$label"
    sleep 1
  done
  printf '\r                                      \r'
  ok "$label ready on :$port"
}

require_bin() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' not found. $2"
}

read_pid() {
  [ -f "$1" ] && cat "$1" || echo ""
}

# -- Logs mode ------------------------------------------------------------------
cmd_logs() {
  [ -d "$LOG_DIR" ] || die "No log directory. Is dev.sh running?"
  local files=""
  for svc in $SERVICES; do
    [ -f "$LOG_DIR/$svc.log" ] && files="$files $LOG_DIR/$svc.log"
  done
  [ -z "$files" ] && die "No service log files in $LOG_DIR"
  tail -f $files
}

# -- Stop mode ------------------------------------------------------------------
cmd_stop() {
  log "Stopping Go services..."
  for svc in $SERVICES; do
    local f="$PID_DIR/$svc.pid"
    if [ -f "$f" ]; then
      local pid
      pid=$(cat "$f")
      kill "$pid" 2>/dev/null && ok "Stopped $svc (pid $pid)" || true
      rm -f "$f"
    fi
  done

  log "Stopping infra (services we started)..."
  for name in anvil clickhouse redis; do
    local f="$PID_DIR/$name.pid"
    if [ -f "$f" ]; then
      local pid
      pid=$(cat "$f")
      kill "$pid" 2>/dev/null && ok "Stopped $name (pid $pid)" || true
      rm -f "$f"
    fi
  done

  if [ -d "$DATA_DIR/postgres" ] && command -v pg_ctl >/dev/null 2>&1; then
    pg_ctl stop -D "$DATA_DIR/postgres" -m fast 2>/dev/null && ok "Postgres stopped" || true
  fi
  ok "Done."
}

# -- Reset ----------------------------------------------------------------------
cmd_reset() {
  warn "Wiping $DATA_DIR and $SEED_MARKER — all local data will be lost."
  printf '  Continue? [y/N] '
  read -r yn
  case "$yn" in
    [Yy]) ;;
    *) info "Aborted."; exit 0 ;;
  esac
  cmd_stop 2>/dev/null || true
  rm -rf "$DATA_DIR" "$SEED_MARKER"
  ok "Data wiped. Run dev.sh again to start fresh."
}

# -- Postgres -------------------------------------------------------------------
start_postgres() {
  if port_open $PG_PORT; then
    ok "Postgres already running on :$PG_PORT — using it"
    return
  fi
  require_bin pg_ctl  "Install: brew install postgresql@16  (or @17, @18)"
  require_bin initdb  "Install: brew install postgresql@16  (or @17, @18)"

  local pgdata="$DATA_DIR/postgres"
  if [ ! -d "$pgdata/base" ]; then
    log "Initialising Postgres data dir..."
    initdb \
      --pgdata="$pgdata" \
      --username=vigil \
      --auth=trust \
      --encoding=UTF8 \
      --locale=C \
      --no-instructions \
      >"$LOG_DIR/postgres-init.log" 2>&1 \
      || die "initdb failed — see $LOG_DIR/postgres-init.log"

    echo "host all all 127.0.0.1/32 trust" >> "$pgdata/pg_hba.conf"
    printf "listen_addresses = '127.0.0.1'\nport = %s\n" "$PG_PORT" >> "$pgdata/postgresql.conf"
  fi

  log "Starting Postgres..."
  pg_ctl start -D "$pgdata" \
    -l "$LOG_DIR/postgres.log" \
    -o "-p $PG_PORT" \
    -w \
    || die "Postgres failed to start — see $LOG_DIR/postgres.log"

  psql -h 127.0.0.1 -p "$PG_PORT" -U vigil -c "CREATE DATABASE vigil;" postgres 2>/dev/null || true
  STARTED_PG=1
  ok "Postgres started on :$PG_PORT"
}

# -- Redis ----------------------------------------------------------------------
start_redis() {
  if port_open $REDIS_PORT; then
    ok "Redis already running on :$REDIS_PORT — using it"
    return
  fi
  require_bin redis-server "Install: brew install redis"

  mkdir -p "$DATA_DIR/redis"
  log "Starting Redis..."
  redis-server \
    --port "$REDIS_PORT" \
    --daemonize yes \
    --logfile "$LOG_DIR/redis.log" \
    --pidfile "$PID_DIR/redis.pid" \
    --dir "$DATA_DIR/redis" \
    --appendonly yes \
    || die "Redis failed to start — see $LOG_DIR/redis.log"

  STARTED_REDIS=1
  ok "Redis started on :$REDIS_PORT"
}

# -- ClickHouse -----------------------------------------------------------------
start_clickhouse() {
  if port_open $CH_HTTP_PORT; then
    ok "ClickHouse already running on :$CH_HTTP_PORT — using it"
    return
  fi
  require_bin clickhouse "Install: brew install clickhouse"

  mkdir -p "$DATA_DIR/clickhouse" "$DATA_DIR/clickhouse-tmp"

  local cfg="$TMP_DIR/clickhouse-config.xml"
  cat > "$cfg" <<XML
<clickhouse>
  <logger>
    <level>warning</level>
    <log>${LOG_DIR}/clickhouse.log</log>
    <errorlog>${LOG_DIR}/clickhouse-error.log</errorlog>
    <console>0</console>
  </logger>
  <http_port>${CH_HTTP_PORT}</http_port>
  <tcp_port>${CH_TCP_PORT}</tcp_port>
  <listen_host>127.0.0.1</listen_host>
  <path>${DATA_DIR}/clickhouse/</path>
  <tmp_path>${DATA_DIR}/clickhouse-tmp/</tmp_path>
  <users_config></users_config>
  <default_profile>default</default_profile>
  <default_database>default</default_database>
  <mark_cache_size>268435456</mark_cache_size>
</clickhouse>
XML

  log "Starting ClickHouse..."
  clickhouse server --config-file="$cfg" >>"$LOG_DIR/clickhouse.log" 2>&1 &
  echo "$!" > "$PID_DIR/clickhouse.pid"
  STARTED_CH=1
  wait_port $CH_HTTP_PORT "ClickHouse" 60

  clickhouse client --host 127.0.0.1 --port "$CH_TCP_PORT" \
    --query "CREATE DATABASE IF NOT EXISTS vigil" 2>/dev/null || true
  ok "ClickHouse started on :$CH_HTTP_PORT / :$CH_TCP_PORT"
}

# -- Anvil (local EVM chain — optional) ----------------------------------------
start_anvil() {
  if port_open $ANVIL_PORT; then
    ok "Anvil already running on :$ANVIL_PORT — using it"
    return
  fi
  if ! command -v anvil >/dev/null 2>&1; then
    warn "anvil not found — skipping local EVM chain (run: foundryup  to install)"
    return
  fi

  log "Starting Anvil (local EVM chain)..."
  anvil \
    --host 127.0.0.1 \
    --port "$ANVIL_PORT" \
    --block-time 2 \
    --chain-id 31337 \
    >>"$LOG_DIR/anvil.log" 2>&1 &
  echo "$!" > "$PID_DIR/anvil.pid"
  STARTED_ANVIL=1
  wait_port $ANVIL_PORT "Anvil" 20
}

# -- Ensure vigil role + database exist ----------------------------------------
# Runs against whatever Postgres/ClickHouse is on the port (ours or pre-existing).
ensure_pg_setup() {
  local current_user
  current_user="$(whoami)"
  psql -h 127.0.0.1 -p "$PG_PORT" -U "$current_user" \
    -c "CREATE ROLE vigil WITH LOGIN;" postgres 2>/dev/null || true
  psql -h 127.0.0.1 -p "$PG_PORT" -U "$current_user" \
    -c "CREATE DATABASE vigil OWNER vigil;" postgres 2>/dev/null || true
}

ensure_ch_setup() {
  clickhouse client --host 127.0.0.1 --port "$CH_TCP_PORT" \
    --query "CREATE DATABASE IF NOT EXISTS vigil" 2>/dev/null || true
}

# -- Migrations -----------------------------------------------------------------
run_migrations() {
  log "Running database migrations..."
  DATABASE_URL="postgres://vigil@127.0.0.1:$PG_PORT/vigil?sslmode=disable" \
  CLICKHOUSE_URL="clickhouse://127.0.0.1:$CH_TCP_PORT/vigil" \
    go run ./services/internal/db/cmd/migrate -direction=up \
    && ok "Migrations applied"
}

# -- Seed -----------------------------------------------------------------------
run_seed() {
  if [ "$OPT_NO_SEED" -eq 1 ]; then
    info "Skipping seed (--no-seed)"
    return
  fi
  if [ -f "$SEED_MARKER" ]; then
    info "Skipping seed (already seeded — delete $SEED_MARKER to re-seed)"
    return
  fi
  log "Seeding demo data..."
  DATABASE_URL="postgres://vigil@127.0.0.1:$PG_PORT/vigil?sslmode=disable" \
  CLICKHOUSE_URL="clickhouse://127.0.0.1:$CH_TCP_PORT/vigil" \
    go run ./services/internal/db/cmd/seed \
    && touch "$SEED_MARKER" && ok "Demo data seeded"
}

# -- Build ----------------------------------------------------------------------
build_services() {
  if [ "$OPT_NO_BUILD" -eq 1 ]; then
    info "Skipping build (--no-build)"
    return
  fi
  log "Building Go services..."
  mkdir -p "$BIN_DIR"
  for svc in $SERVICES; do
    printf '  building %-14s' "$svc..."
    go build -o "$BIN_DIR/vigil-$svc" "./services/$svc/cmd/$svc" \
      && echo "ok" || die "build failed for $svc"
  done
  ok "All services built"
}

# -- Start a Go service ---------------------------------------------------------
start_service() {
  local svc=$1 color=$2
  local port
  port=$(svc_port "$svc")
  local log_file="$LOG_DIR/$svc.log"

  env \
    ENV=local \
    PORT="$port" \
    LOG_LEVEL=info \
    DATABASE_URL="postgres://vigil@127.0.0.1:$PG_PORT/vigil?sslmode=disable" \
    POSTGRES_DSN="postgres://vigil@127.0.0.1:$PG_PORT/vigil?sslmode=disable" \
    CLICKHOUSE_URL="clickhouse://127.0.0.1:$CH_TCP_PORT/vigil" \
    CLICKHOUSE_DSN="clickhouse://127.0.0.1:$CH_TCP_PORT/vigil" \
    REDIS_URL="redis://127.0.0.1:$REDIS_PORT" \
    FLIGHTCHECK_URL="http://127.0.0.1:$(svc_port flightcheck)" \
    INGEST_URL="http://127.0.0.1:$(svc_port ingest)" \
    OTEL_EXPORTER_OTLP_ENDPOINT="" \
    POSTHOG_API_KEY="${POSTHOG_API_KEY:-}" \
    ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
    SLACK_WEBHOOK_URL="${SLACK_WEBHOOK_URL:-}" \
    VIGIL_TELEMETRY=0 \
    "$BIN_DIR/vigil-$svc" \
    > >(while IFS= read -r line; do
         printf "%b[%s]%b %s\n" "$color" "$svc" "$RESET" "$line" | tee -a "$log_file"
       done) \
    2> >(while IFS= read -r line; do
           printf "%b[%s]%b %b%s%b\n" "$color" "$svc" "$RESET" "$RED" "$line" "$RESET" \
             | tee -a "$log_file" >&2
         done) &

  echo "$!" > "$PID_DIR/$svc.pid"
  ok "Started vigil-$svc on :$port (pid $!)"
}

# -- Shutdown handler -----------------------------------------------------------
shutdown() {
  printf "\n"
  log "Shutting down Go services..."
  for svc in $SERVICES; do
    local f="$PID_DIR/$svc.pid"
    if [ -f "$f" ]; then
      kill "$(cat "$f")" 2>/dev/null || true
      rm -f "$f"
    fi
  done

  if [ "$OPT_KEEP_INFRA" -eq 1 ]; then
    info "Leaving infra running (--keep-infra)."
    ok "Done."; exit 0
  fi

  if [ "$STARTED_ANVIL" -eq 1 ] && [ -f "$PID_DIR/anvil.pid" ]; then
    kill "$(cat "$PID_DIR/anvil.pid")" 2>/dev/null || true
    rm -f "$PID_DIR/anvil.pid"
    ok "Anvil stopped"
  fi
  if [ "$STARTED_CH" -eq 1 ] && [ -f "$PID_DIR/clickhouse.pid" ]; then
    kill "$(cat "$PID_DIR/clickhouse.pid")" 2>/dev/null || true
    rm -f "$PID_DIR/clickhouse.pid"
    ok "ClickHouse stopped"
  fi
  if [ "$STARTED_REDIS" -eq 1 ]; then
    redis-cli -p "$REDIS_PORT" shutdown nosave 2>/dev/null || true
    ok "Redis stopped"
  fi
  if [ "$STARTED_PG" -eq 1 ]; then
    pg_ctl stop -D "$DATA_DIR/postgres" -m fast 2>/dev/null || true
    ok "Postgres stopped"
  fi
  ok "Done."
  exit 0
}

# ══════════════════════════════════════════════════════════════════════════════
# Entry point
# ══════════════════════════════════════════════════════════════════════════════
[ "$OPT_STOP"  -eq 1 ] && { cmd_stop;  exit 0; }
[ "$OPT_LOGS"  -eq 1 ] && { cmd_logs;  exit 0; }
[ "$OPT_RESET" -eq 1 ] && { cmd_reset; exit 0; }

require_bin go "Install: https://go.dev/dl/"
mkdir -p "$DATA_DIR" "$LOG_DIR" "$PID_DIR"

# Kill any leftover service processes from a previous run (by PID file and by port)
for svc in $SERVICES; do
  pf="$PID_DIR/$svc.pid"
  if [ -f "$pf" ]; then
    kill "$(cat "$pf")" 2>/dev/null || true
    rm -f "$pf"
  fi
done
for svc in $SERVICES; do
  port=$(svc_port "$svc")
  if command -v lsof >/dev/null 2>&1; then
    pids=$(lsof -ti "tcp:$port" 2>/dev/null) || true
    [ -n "$pids" ] && kill $pids 2>/dev/null || true
  fi
done

printf "${BOLD}"
printf "  ╔══════════════════════════════════════════╗\n"
printf "  ║         Vigil — local dev stack          ║\n"
printf "  ╚══════════════════════════════════════════╝\n"
printf "${RESET}\n"

# -- 1. Infra -------------------------------------------------------------------
log "Starting infra..."
start_postgres
start_redis
start_clickhouse
start_anvil

wait_port "$PG_PORT"    "Postgres" 10
wait_port "$REDIS_PORT" "Redis"    10

ensure_pg_setup
ensure_ch_setup

# -- 2. Migrate + seed ----------------------------------------------------------
run_migrations
run_seed

if [ "$OPT_INFRA_ONLY" -eq 1 ]; then
  printf "\n"
  info "Infra ready (--infra-only). Connection strings:"
  info "  DATABASE_URL=postgres://vigil@127.0.0.1:$PG_PORT/vigil?sslmode=disable"
  info "  CLICKHOUSE_URL=clickhouse://127.0.0.1:$CH_TCP_PORT/vigil"
  info "  REDIS_URL=redis://127.0.0.1:$REDIS_PORT"
  exit 0
fi

# -- 3. Build + start services --------------------------------------------------
build_services

trap shutdown INT TERM

log "Starting Go services..."
i=0
for svc in $SERVICES; do
  start_service "$svc" "${SVC_COLORS[$i]}"
  i=$((i + 1))
done

printf "\n"
info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
info "Services:"
for svc in $SERVICES; do
  port=$(svc_port "$svc")
  info "  vigil-$svc        →  http://127.0.0.1:$port"
done
info ""
info "Infra:"
info "  Postgres    →  127.0.0.1:$PG_PORT   (user: vigil, no password)"
info "  Redis       →  127.0.0.1:$REDIS_PORT"
info "  ClickHouse  →  http://127.0.0.1:$CH_HTTP_PORT  (HTTP UI)"
port_open "$ANVIL_PORT" && info "  Anvil       →  127.0.0.1:$ANVIL_PORT  (chain-id 31337)"
info ""
info "Logs:  tail -f $LOG_DIR/<service>.log"
info "       ./scripts/dev.sh --logs    (all at once)"
info ""
info "Press Ctrl-C to stop everything."
info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
printf "\n"

# -- 4. Watch for unexpected exits ----------------------------------------------
while true; do
  for svc in $SERVICES; do
    pf="$PID_DIR/$svc.pid"
    if [ -f "$pf" ]; then
      spid=$(cat "$pf")
      if ! kill -0 "$spid" 2>/dev/null; then
        warn "vigil-$svc (pid $spid) exited unexpectedly — see $LOG_DIR/$svc.log"
        rm -f "$pf"
      fi
    fi
  done
  sleep 5
done
