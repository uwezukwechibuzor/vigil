/** OTel tracing helpers for Vigil agent runs. */

/** Options for initTracing. */
export interface TraceInitOptions {
  /** OTLP endpoint, defaults to VIGIL_OTLP_ENDPOINT env var or https://ingest.vigil.dev/v1/traces */
  endpoint?: string;
  /** API key for auth, defaults to VIGIL_API_KEY env var */
  apiKey?: string;
  /** Service name for the OTel resource */
  serviceName: string;
}

/**
 * Initialize OTel tracing to export to the Vigil OTLP endpoint.
 * Call once at application startup. Requires @opentelemetry packages to be installed.
 */
export function initTracing(opts: TraceInitOptions): void {
  void (async () => {
    try {
      // Dynamic imports so the SDK works without OTel installed
      const [otelApi, sdkNode, otlpExporter, semConv] = await Promise.all([
        import("@opentelemetry/api"),
        import("@opentelemetry/sdk-trace-node"),
        import("@opentelemetry/exporter-trace-otlp-http"),
        import("@opentelemetry/semantic-conventions"),
      ]);

      const endpoint =
        opts.endpoint ??
        (typeof process !== "undefined" ? process.env["VIGIL_OTLP_ENDPOINT"] : undefined) ??
        "https://ingest.vigil.dev/v1/traces";

      const apiKey =
        opts.apiKey ?? (typeof process !== "undefined" ? process.env["VIGIL_API_KEY"] : undefined);

      const headers: Record<string, string> = {};
      if (apiKey) {
        headers["Authorization"] = `Bearer ${apiKey}`;
      }

      const exporter = new otlpExporter.OTLPTraceExporter({ url: endpoint, headers });

      // Build resource with service.name attribute
      const resourceAttrs: Record<string, string> = {
        [semConv.ATTR_SERVICE_NAME]: opts.serviceName,
      };
      // Use SimpleSpanProcessor as fallback if BatchSpanProcessor not available
      const provider = new sdkNode.NodeTracerProvider({
        resource: { attributes: resourceAttrs } as unknown as InstanceType<
          typeof sdkNode.NodeTracerProvider
        >["resource"],
        spanProcessors: [new sdkNode.BatchSpanProcessor(exporter)],
      });
      provider.register();
      otelApi.trace.setGlobalTracerProvider(provider);
    } catch {
      // OTel packages not installed — tracing is a no-op
    }
  })();
}

/** A span-like interface (subset of OTel Span). */
export interface SpanLike {
  /** Set an attribute on the span. */
  setAttribute(key: string, value: string | number | boolean): void;
  /** Set the span status. */
  setStatus(status: { code: number; message?: string }): void;
  /** End the span. */
  end(): void;
  /** Get the span context. */
  spanContext(): { traceId: string };
}

const noopSpan: SpanLike = {
  setAttribute: () => {},
  setStatus: () => {},
  end: () => {},
  spanContext: () => ({ traceId: crypto.randomUUID().replace(/-/g, "") }),
};

/** Represents a single agent run with tracing support. */
export class Run {
  private readonly _runId: string;
  private readonly _traceId: string;
  private readonly _rootSpan: SpanLike;

  constructor(runId: string, traceId: string, rootSpan: SpanLike) {
    this._runId = runId;
    this._traceId = traceId;
    this._rootSpan = rootSpan;
  }

  /** The unique run identifier. */
  get runId(): string {
    return this._runId;
  }

  /** The OTel trace ID. */
  get traceId(): string {
    return this._traceId;
  }

  /**
   * Wrap an LLM call with gen_ai semantic convention attributes.
   */
  async tracedLLMCall<T>(
    opts: {
      model: string;
      system?: string;
      maxTokens?: number;
    },
    fn: (span: SpanLike) => Promise<T>,
  ): Promise<T> {
    try {
      const otelApi = await import("@opentelemetry/api");
      const tracer = otelApi.trace.getTracer("vigil-sdk");
      return await tracer.startActiveSpan(
        `gen_ai.${opts.model}`,
        { kind: otelApi.SpanKind.CLIENT },
        async (span) => {
          span.setAttribute("gen_ai.operation.name", "chat");
          span.setAttribute("gen_ai.system", "openai");
          span.setAttribute("gen_ai.request.model", opts.model);
          if (opts.system !== undefined) span.setAttribute("gen_ai.system.prompt", opts.system);
          if (opts.maxTokens !== undefined)
            span.setAttribute("gen_ai.request.max_tokens", opts.maxTokens);
          try {
            const result = await fn(span);
            span.setStatus({ code: 1 });
            return result;
          } catch (err) {
            span.setStatus({ code: 2, message: err instanceof Error ? err.message : String(err) });
            throw err;
          } finally {
            span.end();
          }
        },
      );
    } catch {
      return fn(noopSpan);
    }
  }

  /**
   * Wrap a tool call with a span.
   */
  async tracedToolCall<T>(name: string, fn: (span: SpanLike) => Promise<T>): Promise<T> {
    try {
      const otelApi = await import("@opentelemetry/api");
      const tracer = otelApi.trace.getTracer("vigil-sdk");
      return await tracer.startActiveSpan(
        `tool.${name}`,
        { kind: otelApi.SpanKind.INTERNAL },
        async (span) => {
          span.setAttribute("tool.name", name);
          try {
            const result = await fn(span);
            span.setStatus({ code: 1 });
            return result;
          } catch (err) {
            span.setStatus({ code: 2, message: err instanceof Error ? err.message : String(err) });
            throw err;
          } finally {
            span.end();
          }
        },
      );
    } catch {
      return fn(noopSpan);
    }
  }

  /** Link the run to an onchain transaction. */
  attachTx(txHash: string, flightCheckId?: string): void {
    this._rootSpan.setAttribute("vigil.tx_hash", txHash);
    if (flightCheckId !== undefined) {
      this._rootSpan.setAttribute("vigil.flight_check_id", flightCheckId);
    }
  }

  /** End the root run span. */
  end(status?: "ok" | "error", error?: Error): void {
    if (status === "error") {
      this._rootSpan.setStatus({
        code: 2,
        message: error?.message ?? "error",
      });
    } else {
      this._rootSpan.setStatus({ code: 1 });
    }
    this._rootSpan.end();
  }
}

/** Options for startRun. */
export interface RunOptions {
  /** Agent identifier. */
  agentId: string;
  /** Human-readable intent description. */
  intent?: string;
}

/**
 * Start a new agent run with an OTel span.
 * Returns a Run instance. Call run.end() when done.
 */
export function startRun(opts: RunOptions): Run {
  const runId = crypto.randomUUID();
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const otelApi = require("@opentelemetry/api") as typeof import("@opentelemetry/api");
    const tracer = otelApi.trace.getTracer("vigil-sdk");
    const attrs: Record<string, string> = {
      "vigil.run_id": runId,
      "vigil.agent_id": opts.agentId,
    };
    if (opts.intent !== undefined) {
      attrs["vigil.intent"] = opts.intent;
    }
    const rootSpan = tracer.startSpan("agent.run", { attributes: attrs });
    const traceId = rootSpan.spanContext().traceId;
    return new Run(runId, traceId, rootSpan);
  } catch {
    return new Run(runId, noopSpan.spanContext().traceId, noopSpan);
  }
}
