export const VERSION = "0.1.0";

export interface VigilClientOptions {
  baseUrl: string;
  apiKey?: string;
}

export class VigilClient {
  constructor(private readonly opts: VigilClientOptions) {}

  get baseUrl(): string {
    return this.opts.baseUrl;
  }
}
