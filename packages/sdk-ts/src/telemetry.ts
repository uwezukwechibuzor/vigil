/**
 * Anonymous, opt-out telemetry for the Vigil TypeScript SDK.
 *
 * What is sent:  surface, version, OS, runtime, method name, success/fail, latency bucket
 * What is NEVER sent:  wallet addresses, private keys, calldata, API keys, file paths, emails
 * Opt out:  VIGIL_TELEMETRY=0  or  DO_NOT_TRACK=1
 *
 * Events are batched and sent at most once per minute.
 */

const SDK_VERSION = "0.1.0";
const BATCH_INTERVAL_MS = 60_000;
const ENDPOINT = "https://app.posthog.com/capture/";
const PH_KEY = "phc_vigil_sdk_anonymous";

// Ethereum address: 0x + 40 hex chars
const ETH_ADDR_RE = /^0x[0-9a-fA-F]{40}$/;
// 32-byte hex (private key)
const HEX32_RE = /^0x[0-9a-fA-F]{64}$/;
// PII-looking keys
const PII_KEYS = new Set([
  "email",
  "password",
  "private_key",
  "mnemonic",
  "seed",
  "address",
  "from",
  "to",
  "data",
  "calldata",
  "tx",
]);

type Props = Record<string, unknown>;
interface QueuedEvent {
  event: string;
  props: Props;
}

class VigilTelemetry {
  private queue: QueuedEvent[] = [];
  private timer: ReturnType<typeof setTimeout> | null = null;
  private notified = false;

  isEnabled(): boolean {
    if (typeof process === "undefined") return false;
    if (process.env["VIGIL_TELEMETRY"] === "0") return false;
    if (process.env["DO_NOT_TRACK"] === "1") return false;
    return true;
  }

  track(event: string, props: Props): void {
    if (!this.isEnabled()) return;
    if (!this.notified) {
      this.notified = true;
      // one-time notice on first event
      process.stderr.write(
        "[vigil] Anonymous telemetry is enabled. Disable with VIGIL_TELEMETRY=0.\n",
      );
    }
    this.queue.push({ event, props: this.sanitize(props) });
    this.scheduleFlush();
  }

  /** Flush immediately (for tests) */
  async flush(): Promise<void> {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
    await this.send(this.queue.splice(0));
  }

  private sanitize(props: Props): Props {
    const out: Props = {};
    for (const [k, v] of Object.entries(props)) {
      if (PII_KEYS.has(k.toLowerCase())) continue;
      if (typeof v === "string" && (ETH_ADDR_RE.test(v) || HEX32_RE.test(v))) continue;
      out[k] = v;
    }
    return out;
  }

  private scheduleFlush(): void {
    if (this.timer) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      void this.send(this.queue.splice(0));
    }, BATCH_INTERVAL_MS);
    // Don't block process exit
    if (this.timer.unref) this.timer.unref();
  }

  private async send(batch: QueuedEvent[]): Promise<void> {
    if (batch.length === 0) return;
    const payload = {
      api_key: PH_KEY,
      batch: batch.map(({ event, props }) => ({
        event,
        properties: {
          ...props,
          $lib: "vigil-sdk-ts",
          $lib_version: SDK_VERSION,
          surface: "sdk_ts",
          distinct_id: this.anonId(),
        },
      })),
    };
    try {
      const ctrl = new AbortController();
      const t = setTimeout(() => ctrl.abort(), 3000);
      await fetch(ENDPOINT, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
        signal: ctrl.signal,
      });
      clearTimeout(t);
    } catch {
      // silent failure — telemetry must never break the caller
    }
  }

  private anonId(): string {
    if (typeof process !== "undefined") {
      return `sdk-ts-${process.platform}-${process.version}`;
    }
    return "sdk-ts-unknown";
  }
}

export const telemetry = new VigilTelemetry();
