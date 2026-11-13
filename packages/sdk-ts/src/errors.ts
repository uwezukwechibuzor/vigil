/** Vigil typed error. */
export class VigilError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly details?: Record<string, unknown>,
    public readonly statusCode?: number,
  ) {
    super(message);
    this.name = "VigilError";
    // Restore prototype chain (TypeScript + extends Error quirk)
    Object.setPrototypeOf(this, new.target.prototype);
  }
}
