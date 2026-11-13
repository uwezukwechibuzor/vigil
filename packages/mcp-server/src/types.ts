export interface ServerOptions {
  apiKey?: string; // defaults to VIGIL_API_KEY env var
  baseUrl?: string; // defaults to https://api.vigil.dev
  transport?: "stdio" | "http";
  port?: number; // for http transport, default 3001
}
