-- Migration 008: add Vigil agent convention columns to spans table.
ALTER TABLE spans ADD COLUMN IF NOT EXISTS vigil_agent_id        String DEFAULT '';
ALTER TABLE spans ADD COLUMN IF NOT EXISTS vigil_run_id          String DEFAULT '';
ALTER TABLE spans ADD COLUMN IF NOT EXISTS vigil_wallet          String DEFAULT '';
ALTER TABLE spans ADD COLUMN IF NOT EXISTS vigil_chain_id        UInt64 DEFAULT 0;
ALTER TABLE spans ADD COLUMN IF NOT EXISTS vigil_tx_hash         String DEFAULT '';
ALTER TABLE spans ADD COLUMN IF NOT EXISTS vigil_flight_check_id String DEFAULT '';
ALTER TABLE spans ADD COLUMN IF NOT EXISTS vigil_intent          String DEFAULT '';
ALTER TABLE spans ADD COLUMN IF NOT EXISTS llm_cost_usd          Float64 DEFAULT 0;
ALTER TABLE spans ADD COLUMN IF NOT EXISTS gas_cost_usd          Float64 DEFAULT 0;
ALTER TABLE spans ADD COLUMN IF NOT EXISTS linked                UInt8 DEFAULT 0;
ALTER TABLE spans ADD COLUMN IF NOT EXISTS silent_failure        UInt8 DEFAULT 0;
