/** Vigil TypeScript SDK — reliability layer for onchain AI agents. */

export { VigilClient } from "./client.js";
export type { VigilClientOptions } from "./client.js";
export { VigilError } from "./errors.js";
export type {
  Verdict,
  HealthStatus,
  FeeLevel,
  EndpointKind,
  IncidentStatus,
  IncidentSeverity,
  AlertChannelKind,
  ListMeta,
  Chain,
  ChainList,
  ChainHealth,
  Endpoint,
  EndpointList,
  EndpointHealth,
  OracleHealth,
  FlightCheckRequest,
  FlightCheckResult,
  RunSummary,
  RunList,
  Span,
  RunDetail,
  Incident,
  IncidentList,
  Policy,
  PolicyList,
  CreatePolicyRequest,
  AlertRule,
  AlertRuleList,
  AlertChannel,
  AlertChannelList,
  APIKey,
  APIKeyList,
  APIKeyCreateResult,
  CreateAPIKeyRequest,
  UsageRecord,
  UsageList,
} from "./types.js";
export { initTracing, startRun, Run } from "./tracing.js";
export type { TraceInitOptions, RunOptions } from "./tracing.js";
export { guardedSend, viemSigner, ethersSigner } from "./guarded.js";
export type { WalletSigner } from "./guarded.js";

export const VERSION = "0.1.0";
