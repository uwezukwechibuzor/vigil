# Vigil Span Conventions

Standard OpenTelemetry attributes used by Vigil agents.

## Vigil Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| vigil.agent.id | string | Unique identifier for the agent instance |
| vigil.run.id | string | Identifier for a single agent execution run |
| vigil.wallet | string | Onchain wallet address (never a private key) |
| vigil.chain_id | int | EVM chain ID |
| vigil.tx.hash | string | Transaction hash after signing |
| vigil.flight_check.id | string | UUID of the flight check that approved this tx |
| vigil.intent | string | Human-readable description of the agent's goal |
| vigil.gas_estimate | int | Estimated gas for the transaction (used for silent failure detection) |

## GenAI Semantic Conventions

Vigil follows the [OpenTelemetry GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/).

| Attribute | Type | Description |
|-----------|------|-------------|
| gen_ai.system | string | LLM provider (e.g. `anthropic`, `openai`) |
| gen_ai.request.model | string | Model name (e.g. `claude-sonnet-4-6`) |
| gen_ai.operation.name | string | Operation type (e.g. `chat`, `completion`) |
| gen_ai.usage.input_tokens | int | Input token count for the LLM call |
| gen_ai.usage.output_tokens | int | Output token count for the LLM call |

## Security

- Never set `vigil.wallet` to a private key. The ingest service will detect and redact 64-hex-char strings.
- The scrubber redacts: Ethereum private keys (64 hex chars), BIP-39 mnemonics (12/24 words), and API keys (`sk-…`).
- `vigil.intent` is scrubbed before storage. Do not embed secrets in intent strings.

## Sampling

Spans are sampled according to `INGEST_SAMPLE_RATE` (default 1.0 = keep all). The following spans are **always kept** regardless of sample rate:

- Spans with `status_code = error`
- Spans with `vigil.tx.hash` set
- Spans with `vigil.flight_check.id` set
