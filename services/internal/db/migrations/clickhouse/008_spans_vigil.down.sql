-- Migration 008 rollback: remove Vigil agent convention columns from spans table.
ALTER TABLE spans DROP COLUMN IF EXISTS vigil_agent_id;
ALTER TABLE spans DROP COLUMN IF EXISTS vigil_run_id;
ALTER TABLE spans DROP COLUMN IF EXISTS vigil_wallet;
ALTER TABLE spans DROP COLUMN IF EXISTS vigil_chain_id;
ALTER TABLE spans DROP COLUMN IF EXISTS vigil_tx_hash;
ALTER TABLE spans DROP COLUMN IF EXISTS vigil_flight_check_id;
ALTER TABLE spans DROP COLUMN IF EXISTS vigil_intent;
ALTER TABLE spans DROP COLUMN IF EXISTS llm_cost_usd;
ALTER TABLE spans DROP COLUMN IF EXISTS gas_cost_usd;
ALTER TABLE spans DROP COLUMN IF EXISTS linked;
ALTER TABLE spans DROP COLUMN IF EXISTS silent_failure;
