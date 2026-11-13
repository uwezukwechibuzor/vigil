# Contributing to Vigil

Thank you for your interest in contributing!

## Development setup

### Prerequisites

- Go 1.23+
- Node.js 22 + pnpm
- Python 3.11+
- Docker + Docker Compose

### Start the full stack

```bash
docker compose up -d          # Postgres, ClickHouse, Redis
make build-go                 # Build all Go services
pnpm install                  # Install TS dependencies
```

### Run tests

```bash
make test                     # All Go unit tests
pnpm -r test                  # All TS unit tests
make lint                     # golangci-lint + ESLint
```

## Submitting changes

1. Fork the repository and create a branch from `main`.
2. Write code that follows the project rules in `CLAUDE.md`.
3. Add or update tests to cover your change.
4. Run `make fmt lint test build` — all checks must pass before submitting.
5. Open a pull request with a clear description of what and why.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add per-chain rate limiting
fix: prevent MAD=0 for constant-value series
docs: add self-hosting guide
chore: upgrade kin-openapi to v0.153.0
```

No task numbers, no AI attribution lines.

## Code style

- **Go**: `gofmt` + `golangci-lint`; structured JSON logs (`slog`); context timeouts on all outbound calls.
- **TypeScript**: ESLint + Prettier; strict null checks.
- **Python**: `ruff` + `mypy`; pydantic v2 for data models.

## Security

- Never hardcode secrets; use env vars.
- Never handle or store private keys.
- Run `gosec` and `trivy` before submitting — zero HIGH/CRITICAL findings required.
- See [SECURITY.md](SECURITY.md) for the vulnerability disclosure process.

## Licence

By contributing you agree that your contributions will be licensed under the same licence as the project.
