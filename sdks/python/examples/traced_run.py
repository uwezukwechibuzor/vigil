"""Example: startRun + traced_llm_call + end."""
from vigil import VigilClient, init_tracing, start_run

init_tracing(service_name="my-agent")

_vigil = VigilClient()
run = start_run(agent_id="agent_abc123", intent="swap USDC on Base")

answer = run.traced_llm_call(
    "claude-3-5-sonnet-20241022",
    lambda span: "I will swap 100 USDC to ETH",
    max_tokens=1024,
)
print("LLM answer:", answer)

run.traced_tool_call(
    "execute_swap",
    lambda span: run.attach_tx("0xtxhash"),
)

run.end("ok")
print("Run complete. run_id:", run.run_id)
