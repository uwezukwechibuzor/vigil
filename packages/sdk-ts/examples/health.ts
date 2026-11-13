// Example: Get chain health for Base (chain_id 8453)
import { VigilClient } from "../src/index.js";

const vigil = new VigilClient(); // reads VIGIL_API_KEY from env

const health = await vigil.chainHealth(8453);
console.log("Base chain health:", health.status, "| fee:", health.fee_level);
