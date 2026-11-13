import { VigilError } from "./errors.js";
import type {
  FlightCheckRequest,
  FlightCheckResult,
  ChainList,
  ChainHealth,
  EndpointList,
  EndpointHealth,
  OracleHealth,
  RunList,
  RunDetail,
  IncidentList,
  PolicyList,
  Policy,
  CreatePolicyRequest,
  UsageList,
  APIKey,
  APIKeyList,
  APIKeyCreateResult,
  CreateAPIKeyRequest,
} from "./types.js";

export interface VigilClientOptions {
  /** API key — defaults to VIGIL_API_KEY env var */
  apiKey?: string;
  /** Base URL — defaults to https://api.vigil.dev */
  baseUrl?: string;
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  query?: Record<string, string | undefined>;
}

const DEFAULT_BASE_URL = "https://api.vigil.dev";
const MAX_RETRIES = 3;

/** Sleep for ms milliseconds. */
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Parse Retry-After header value to milliseconds. */
function parseRetryAfter(header: string | null): number | null {
  if (!header) return null;
  const seconds = parseFloat(header);
  if (!isNaN(seconds)) return Math.ceil(seconds * 1000);
  const date = new Date(header);
  if (!isNaN(date.getTime())) {
    return Math.max(0, date.getTime() - Date.now());
  }
  return null;
}

/** Vigil API client. */
export class VigilClient {
  private readonly apiKey: string | undefined;
  private readonly baseUrl: string;

  constructor(opts: VigilClientOptions = {}) {
    this.apiKey =
      opts.apiKey ??
      (typeof process !== "undefined" ? process.env["VIGIL_API_KEY"] : undefined);
    this.baseUrl = opts.baseUrl ?? DEFAULT_BASE_URL;
  }

  // ---- Core HTTP ----

  private buildUrl(path: string, query?: Record<string, string | undefined>): string {
    const url = new URL(`/v1${path}`, this.baseUrl);
    if (query) {
      for (const [k, v] of Object.entries(query)) {
        if (v !== undefined) url.searchParams.set(k, v);
      }
    }
    return url.toString();
  }

  private buildHeaders(isPost: boolean): Record<string, string> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      "User-Agent": "vigil-ts/0.1.0",
    };
    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }
    if (isPost) {
      headers["X-Idempotency-Key"] = crypto.randomUUID();
    }
    return headers;
  }

  private async request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    const method = opts.method ?? "GET";
    const isPost = method === "POST";
    const url = this.buildUrl(path, opts.query);

    for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
      const headers = this.buildHeaders(isPost);
      const fetchInit: RequestInit = {
        method,
        headers,
      };
      if (opts.body !== undefined) {
        fetchInit.body = JSON.stringify(opts.body);
      }

      let resp: Response;
      try {
        resp = await fetch(url, fetchInit);
      } catch (err) {
        throw new VigilError(
          "network_error",
          err instanceof Error ? err.message : "Network error",
        );
      }

      // Retry on 429 or 5xx
      if ((resp.status === 429 || resp.status >= 500) && attempt < MAX_RETRIES) {
        const retryAfter = parseRetryAfter(resp.headers.get("Retry-After"));
        const jitter = Math.random() * 100;
        const backoff = retryAfter ?? Math.min(1000 * Math.pow(2, attempt) + jitter, 30000);
        await sleep(backoff);
        continue;
      }

      if (!resp.ok) {
        let errBody: { error?: { code?: string; message?: string; details?: Record<string, unknown> } } = {};
        try {
          errBody = (await resp.json()) as typeof errBody;
        } catch {
          // ignore parse error
        }
        const errInfo = errBody.error ?? {};
        throw new VigilError(
          errInfo.code ?? "api_error",
          errInfo.message ?? `HTTP ${resp.status}`,
          errInfo.details,
          resp.status,
        );
      }

      if (resp.status === 204) {
        return undefined as unknown as T;
      }

      return resp.json() as Promise<T>;
    }

    throw new VigilError("max_retries", "Max retries exceeded");
  }

  // ---- Flight Checks ----

  /**
   * Run a pre-transaction flight check.
   * Never throws on "warn" verdict — check result.verdict.
   */
  async flightCheck(
    tx: FlightCheckRequest,
    opts?: { onWarn?: (r: FlightCheckResult) => void | Promise<void> },
  ): Promise<FlightCheckResult> {
    const result = await this.request<FlightCheckResult>("/flight-checks", {
      method: "POST",
      body: tx,
    });
    if (result.verdict === "warn" && opts?.onWarn) {
      await opts.onWarn(result);
    }
    return result;
  }

  /** Get a flight check by ID. */
  async getFlightCheck(id: string): Promise<FlightCheckResult> {
    return this.request<FlightCheckResult>(`/flight-checks/${id}`);
  }

  // ---- Chains ----

  /** List all supported chains. */
  async listChains(): Promise<ChainList> {
    return this.request<ChainList>("/chains");
  }

  /** Get health for a specific chain. */
  async chainHealth(chainId: number | bigint): Promise<ChainHealth> {
    return this.request<ChainHealth>(`/chains/${String(chainId)}/health`);
  }

  // ---- Endpoints ----

  /** List endpoints for a project. */
  async listEndpoints(projectId: string): Promise<EndpointList> {
    return this.request<EndpointList>("/endpoints", {
      query: { project_id: projectId },
    });
  }

  /** Get endpoint health. */
  async endpointHealth(id: string): Promise<EndpointHealth> {
    return this.request<EndpointHealth>(`/endpoints/${id}/health`);
  }

  // ---- Oracles ----

  /** Get oracle health for a feed address. */
  async oracleHealth(feed: string): Promise<OracleHealth> {
    return this.request<OracleHealth>(`/oracles/${encodeURIComponent(feed)}/health`);
  }

  // ---- Runs ----

  /** List agent runs. */
  async listRuns(params?: {
    agentId?: string;
    projectId?: string;
    cursor?: string;
  }): Promise<RunList> {
    return this.request<RunList>("/runs", {
      query: {
        agent_id: params?.agentId,
        project_id: params?.projectId,
        cursor: params?.cursor,
      },
    });
  }

  /** Get a run by ID. */
  async getRun(id: string): Promise<RunDetail> {
    return this.request<RunDetail>(`/runs/${id}`);
  }

  // ---- Incidents ----

  /** List incidents. */
  async listIncidents(params?: {
    projectId?: string;
    status?: string;
  }): Promise<IncidentList> {
    return this.request<IncidentList>("/incidents", {
      query: {
        project_id: params?.projectId,
        status: params?.status,
      },
    });
  }

  // ---- Policies ----

  /** List policies for a project. */
  async listPolicies(projectId: string): Promise<PolicyList> {
    return this.request<PolicyList>("/policies", {
      query: { project_id: projectId },
    });
  }

  /** Create a policy. */
  async createPolicy(req: CreatePolicyRequest): Promise<Policy> {
    return this.request<Policy>("/policies", {
      method: "POST",
      body: req,
    });
  }

  // ---- Usage ----

  /** Get usage records. */
  async getUsage(params: { orgId: string; from: string; to: string }): Promise<UsageList> {
    return this.request<UsageList>("/usage", {
      query: { org_id: params.orgId, from: params.from, to: params.to },
    });
  }

  // ---- API Keys ----

  /** Create a new API key. Returns key metadata and plaintext secret (shown once). */
  async createAPIKey(req: CreateAPIKeyRequest): Promise<APIKeyCreateResult> {
    const result = await this.request<APIKey & { key: string }>("/api-keys", {
      method: "POST",
      body: req,
    });
    const { key: secret, ...key } = result;
    return { key: key as APIKey, secret };
  }

  /** List API keys for an org. */
  async listAPIKeys(orgId: string): Promise<APIKeyList> {
    return this.request<APIKeyList>("/api-keys", {
      query: { org_id: orgId },
    });
  }

  /** Revoke an API key. */
  async revokeAPIKey(id: string): Promise<void> {
    return this.request<void>(`/api-keys/${id}`, { method: "DELETE" });
  }
}
