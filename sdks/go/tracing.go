package vigil

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultOTLPEndpoint = "https://ingest.vigil.dev"
	tracerName          = "vigil-go"
)

// TraceOptions configures OTel export to Vigil.
type TraceOptions struct {
	// ServiceName is required — identifies this service in traces.
	ServiceName string
	// Endpoint is the OTLP HTTP base URL. Defaults to VIGIL_OTLP_ENDPOINT or https://ingest.vigil.dev.
	Endpoint string
	// APIKey is the Vigil API key. Defaults to VIGIL_API_KEY.
	APIKey string
}

// InitTracing configures OTel to export to the Vigil OTLP endpoint.
// Returns a shutdown function that should be deferred.
func InitTracing(ctx context.Context, opts TraceOptions) (shutdown func(context.Context) error, err error) {
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv("VIGIL_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = defaultOTLPEndpoint
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("VIGIL_API_KEY")
	}

	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(endpoint + "/v1/traces"),
	}
	if apiKey != "" {
		exporterOpts = append(exporterOpts, otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + apiKey,
		}))
	}

	exporter, err := otlptracehttp.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("vigil: create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(opts.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("vigil: create otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// RunOptions configures a new agent run.
type RunOptions struct {
	// AgentID identifies the agent.
	AgentID string
	// Intent is a human-readable description of what the agent is doing.
	Intent string
}

// LLMCallOptions configures an LLM span.
type LLMCallOptions struct {
	// Model is the gen_ai model identifier.
	Model string
	// System is the system prompt.
	System string
	// MaxTokens is the max token limit.
	MaxTokens int
}

// Run represents an agent run with OTel tracing.
type Run struct {
	// RunID is the unique run identifier.
	RunID string
	// TraceID is the OTel trace ID.
	TraceID  string
	rootSpan trace.Span
	tracer   trace.Tracer
	ctx      context.Context
}

// StartRun begins a new agent run. Returns an updated context (with the root span) and the Run.
func StartRun(ctx context.Context, opts RunOptions) (context.Context, *Run) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	runID := newUUID()

	ctx, rootSpan := tracer.Start(ctx, "agent.run",
		trace.WithAttributes(
			attribute.String("vigil.run_id", runID),
			attribute.String("vigil.agent_id", opts.AgentID),
		),
	)
	if opts.Intent != "" {
		rootSpan.SetAttributes(attribute.String("vigil.intent", opts.Intent))
	}

	traceID := rootSpan.SpanContext().TraceID().String()

	r := &Run{
		RunID:    runID,
		TraceID:  traceID,
		rootSpan: rootSpan,
		tracer:   tracer,
		ctx:      ctx,
	}
	return ctx, r
}

// TracedLLMCall wraps an LLM call with gen_ai semantic convention attributes.
func (r *Run) TracedLLMCall(ctx context.Context, opts LLMCallOptions, fn func(context.Context) error) error {
	_, span := r.tracer.Start(ctx, "gen_ai."+opts.Model,
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", "openai"),
			attribute.String("gen_ai.request.model", opts.Model),
		),
	)
	if opts.System != "" {
		span.SetAttributes(attribute.String("gen_ai.system.prompt", opts.System))
	}
	if opts.MaxTokens > 0 {
		span.SetAttributes(attribute.Int("gen_ai.request.max_tokens", opts.MaxTokens))
	}
	defer span.End()

	if err := fn(ctx); err != nil {
		span.RecordError(err)
		return err
	}
	return nil
}

// TracedToolCall wraps a tool call with a span.
func (r *Run) TracedToolCall(ctx context.Context, name string, fn func(context.Context) error) error {
	_, span := r.tracer.Start(ctx, "tool."+name,
		trace.WithAttributes(attribute.String("tool.name", name)),
	)
	defer span.End()

	if err := fn(ctx); err != nil {
		span.RecordError(err)
		return err
	}
	return nil
}

// AttachTx links the run to an onchain transaction.
func (r *Run) AttachTx(txHash string, flightCheckID string) {
	r.rootSpan.SetAttributes(attribute.String("vigil.tx_hash", txHash))
	if flightCheckID != "" {
		r.rootSpan.SetAttributes(attribute.String("vigil.flight_check_id", flightCheckID))
	}
}

// End completes the run span. Pass a non-nil error to mark it as failed.
func (r *Run) End(_ context.Context, err error) {
	if err != nil {
		r.rootSpan.RecordError(err)
	}
	r.rootSpan.End()
}
