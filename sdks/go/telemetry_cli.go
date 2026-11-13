package vigil

// TelTrackCLI fires an anonymous cli_command_run event.
// command and subcommand are sanitized (never include user args).
func TelTrackCLI(command, subcommand string, success bool, latencyBucket string) {
	telTrack("cli_command_run", map[string]any{
		"command":        command,
		"subcommand":     subcommand,
		"success":        success,
		"latency_bucket": latencyBucket,
		"surface":        "cli",
	})
}

// LatencyBucket maps a latency in ms to a string bucket for anonymous telemetry.
func LatencyBucket(ms int) string {
	switch {
	case ms < 100:
		return "lt100ms"
	case ms < 500:
		return "lt500ms"
	case ms < 2000:
		return "lt2s"
	case ms < 10000:
		return "lt10s"
	default:
		return "gt10s"
	}
}
