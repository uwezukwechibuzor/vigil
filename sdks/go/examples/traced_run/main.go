//go:build ignore

// Example: StartRun + TracedLLMCall + End.
package main

import (
	"context"
	"fmt"
	"log"

	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func main() {
	ctx := context.Background()

	shutdown, err := vigil.InitTracing(ctx, vigil.TraceOptions{
		ServiceName: "my-agent",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Println("shutdown error:", err)
		}
	}()

	ctx, run := vigil.StartRun(ctx, vigil.RunOptions{
		AgentID: "agent_abc123",
		Intent:  "swap USDC on Base",
	})

	err = run.TracedLLMCall(ctx, vigil.LLMCallOptions{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
	}, func(c context.Context) error {
		fmt.Println("LLM: I will swap 100 USDC to ETH")
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	err = run.TracedToolCall(ctx, "execute_swap", func(c context.Context) error {
		run.AttachTx("0xtxhash", "")
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	run.End(ctx, nil)
	fmt.Printf("Run complete. run_id=%s trace_id=%s\n", run.RunID, run.TraceID)
}
