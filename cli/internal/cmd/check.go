package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <tx.json | ->",
		Short: "Run a pre-transaction flight check",
		Long: `Run a Vigil flight check for a transaction.

Pass a JSON file path or - to read from stdin.
Exit codes: 0=pass/warn  1=fail  2=error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error
			if args[0] == "-" {
				data, err = io.ReadAll(os.Stdin)
			} else {
				data, err = os.ReadFile(args[0]) //nolint:gosec
			}
			if err != nil {
				return fmt.Errorf("read tx: %w", err)
			}

			var req vigil.FlightCheckRequest
			if err := json.Unmarshal(data, &req); err != nil {
				return fmt.Errorf("parse tx json: %w", err)
			}

			client := newClient()
			start := time.Now()
			result, err := client.FlightCheck(cmd.Context(), req)
			latencyMs := int(time.Since(start).Milliseconds())
			success := err == nil
			latencyBucket := vigil.LatencyBucket(latencyMs)
			vigil.TelTrackCLI("flight-check", "run", success, latencyBucket)
			if err != nil {
				return fmt.Errorf("flight check: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			verdictSymbol := map[vigil.Verdict]string{
				vigil.VerdictPass: "PASS",
				vigil.VerdictWarn: "WARN",
				vigil.VerdictFail: "FAIL",
			}[result.Verdict]

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "verdict:  %s\n", verdictSymbol)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "score:    %.2f\n", result.Score)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "id:       %s\n", result.ID)
			if result.LatencyMS != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "latency:  %dms\n", *result.LatencyMS)
			}

			if result.Verdict == vigil.VerdictFail {
				// Exit code 1 for fail
				os.Exit(1)
			}
			return nil
		},
	}
}
