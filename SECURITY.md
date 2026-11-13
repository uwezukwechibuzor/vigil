# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| `main` (latest) | ✅ |
| Older releases | ❌ |

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Email **uwezukwechibuzor@yahoo.com** with:

1. A description of the vulnerability and affected component(s).
2. Steps to reproduce (or a proof-of-concept).
3. The potential impact.
4. Any suggested mitigations (optional).

We will acknowledge receipt within **48 hours** and aim to provide a fix or workaround within **14 days** for critical issues. We will credit reporters in release notes unless you request otherwise.

## Scope

In scope:
- Authentication / authorisation bypass
- SSRF / server-side injection
- Data exposure across tenant boundaries
- Denial-of-service via crafted payloads
- Dependency vulnerabilities in production code

Out of scope:
- Vulnerabilities in development-only tools
- Issues requiring physical access to infrastructure
- Social engineering
- Vigil explicitly never handles or stores private keys — private key security is the caller's responsibility

## Security design principles

- **No private keys**: Vigil never handles, stores, or transmits private keys.
- **Risk signals, not guarantees**: Flight checks provide risk signals; they do not guarantee transaction safety.
- **Telemetry privacy**: SDK telemetry is anonymous and can be disabled with `VIGIL_TELEMETRY=0` or `DO_NOT_TRACK=1`.
- **12-factor config**: Secrets are loaded from environment variables via AWS Secrets Manager; never hardcoded.

## Responsible disclosure

We follow coordinated disclosure: we ask for 90 days before public disclosure of unpatched vulnerabilities. For critical issues with active exploits we may request shorter embargo periods.
