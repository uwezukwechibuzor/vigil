"""OTel tracing helpers for Vigil agent runs."""

from __future__ import annotations

import os
import uuid
from collections.abc import Callable
from typing import TypeVar

from opentelemetry import context, trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.trace import Span, Tracer

T = TypeVar("T")

_DEFAULT_OTLP_ENDPOINT = "https://ingest.vigil.dev/v1/traces"


def init_tracing(
    *,
    endpoint: str | None = None,
    api_key: str | None = None,
    service_name: str,
) -> None:
    """Configure OTel to export to the Vigil OTLP endpoint."""
    resolved_endpoint = (
        endpoint
        or os.environ.get("VIGIL_OTLP_ENDPOINT")
        or _DEFAULT_OTLP_ENDPOINT
    )
    resolved_key = api_key or os.environ.get("VIGIL_API_KEY")

    headers: dict[str, str] = {}
    if resolved_key:
        headers["Authorization"] = f"Bearer {resolved_key}"

    exporter = OTLPSpanExporter(endpoint=resolved_endpoint, headers=headers)
    provider = TracerProvider(resource=Resource(attributes={SERVICE_NAME: service_name}))
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)


class Run:
    """Represents a single agent run with OTel tracing."""

    def __init__(self, tracer: Tracer, run_id: str, trace_id: str, _root_span: Span) -> None:
        self._tracer = tracer
        self.run_id = run_id
        self.trace_id = trace_id
        self._root_span = _root_span
        self._ctx = trace.set_span_in_context(_root_span)

    def traced_llm_call(
        self,
        model: str,
        fn: Callable[..., T],
        *,
        max_tokens: int | None = None,
    ) -> T:
        """Wrap an LLM call with gen_ai semantic convention attributes."""
        with self._tracer.start_as_current_span(
            f"gen_ai.{model}",
            context=self._ctx,
        ) as span:
            span.set_attribute("gen_ai.operation.name", "chat")
            span.set_attribute("gen_ai.system", "openai")
            span.set_attribute("gen_ai.request.model", model)
            if max_tokens is not None:
                span.set_attribute("gen_ai.request.max_tokens", max_tokens)
            try:
                result: T = fn(span)
                span.set_status(trace.StatusCode.OK)
                return result
            except Exception as exc:
                span.set_status(trace.StatusCode.ERROR, str(exc))
                raise

    def traced_tool_call(self, name: str, fn: Callable[..., T]) -> T:
        """Wrap a tool call with a span."""
        with self._tracer.start_as_current_span(
            f"tool.{name}",
            context=self._ctx,
        ) as span:
            span.set_attribute("tool.name", name)
            try:
                result: T = fn(span)
                span.set_status(trace.StatusCode.OK)
                return result
            except Exception as exc:
                span.set_status(trace.StatusCode.ERROR, str(exc))
                raise

    def attach_tx(self, tx_hash: str, flight_check_id: str | None = None) -> None:
        """Link the run to an onchain transaction."""
        self._root_span.set_attribute("vigil.tx_hash", tx_hash)
        if flight_check_id:
            self._root_span.set_attribute("vigil.flight_check_id", flight_check_id)

    def end(self, status: str = "ok", error: Exception | None = None) -> None:
        """End the run span."""
        if status == "error":
            self._root_span.set_status(
                trace.StatusCode.ERROR,
                str(error) if error else "error",
            )
        else:
            self._root_span.set_status(trace.StatusCode.OK)
        self._root_span.end()


def start_run(*, agent_id: str, intent: str | None = None) -> Run:
    """Start a new agent run. Returns a Run instance."""
    run_id = str(uuid.uuid4())
    tracer: Tracer = trace.get_tracer("vigil-sdk")
    root_span: Span = tracer.start_span(
        "agent.run",
        attributes={
            "vigil.run_id": run_id,
            "vigil.agent_id": agent_id,
            **({"vigil.intent": intent} if intent else {}),
        },
    )
    ctx = trace.set_span_in_context(root_span)
    # Store context so child spans attach correctly
    _ = context.attach(ctx)
    trace_id_int = root_span.get_span_context().trace_id
    trace_id_hex = format(trace_id_int, "032x")
    return Run(tracer, run_id, trace_id_hex, root_span)
