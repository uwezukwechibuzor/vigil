import { VigilClient } from "./client.js";
import { VigilError } from "./errors.js";
import type { FlightCheckRequest, FlightCheckResult } from "./types.js";
import type { Run } from "./tracing.js";

/** Abstract wallet signer. Accepts viem WalletClient or ethers Signer via duck-typing. */
export interface WalletSigner {
  sendTransaction(tx: {
    to: string;
    data?: string;
    value?: bigint;
    chainId?: number;
  }): Promise<string>;
}

/**
 * Wrap a viem WalletClient as a WalletSigner.
 * Uses duck-typing — no hard dependency on viem.
 */
export function viemSigner(walletClient: unknown): WalletSigner {
  const wc = walletClient as {
    sendTransaction: (tx: {
      to: `0x${string}`;
      data?: `0x${string}`;
      value?: bigint;
      chainId?: number;
    }) => Promise<`0x${string}`>;
  };
  return {
    sendTransaction: async (tx) => {
      const sendTx: {
        to: `0x${string}`;
        data?: `0x${string}`;
        value?: bigint;
        chainId?: number;
      } = { to: tx.to as `0x${string}` };
      if (tx.data !== undefined) sendTx.data = tx.data as `0x${string}`;
      if (tx.value !== undefined) sendTx.value = tx.value;
      if (tx.chainId !== undefined) sendTx.chainId = tx.chainId;
      const hash = await wc.sendTransaction(sendTx);
      return hash;
    },
  };
}

/**
 * Wrap an ethers v6 Signer as a WalletSigner.
 * Uses duck-typing — no hard dependency on ethers.
 */
export function ethersSigner(signer: unknown): WalletSigner {
  const s = signer as {
    sendTransaction: (tx: {
      to: string;
      data?: string;
      value?: bigint;
      chainId?: number;
    }) => Promise<{ hash: string }>;
  };
  return {
    sendTransaction: async (tx) => {
      const response = await s.sendTransaction(tx);
      return response.hash;
    },
  };
}

/**
 * Run a flight check then send the transaction if the verdict is not "fail".
 * - Throws VigilError with code "flight_check_failed" on "fail" verdict.
 * - Calls onWarn on "warn" verdict (does not throw).
 * - Attaches tx hash to run span if run is provided.
 */
export async function guardedSend(
  client: VigilClient,
  signer: WalletSigner,
  tx: FlightCheckRequest,
  opts?: {
    onWarn?: (result: FlightCheckResult) => Promise<void>;
    run?: Run;
  },
): Promise<{ txHash: string; flightCheckId: string }> {
  const flightOpts = opts?.onWarn !== undefined
    ? { onWarn: opts.onWarn }
    : {};
  const result = await client.flightCheck(tx, flightOpts);

  if (result.verdict === "fail") {
    throw new VigilError(
      "flight_check_failed",
      `Flight check failed with score ${result.score}`,
      { checks: result.checks, flight_check_id: result.id },
    );
  }

  const sendTx: { to: string; data?: string; value?: bigint; chainId?: number } = {
    to: tx.to,
  };
  if (tx.data !== undefined) sendTx.data = tx.data;
  if (tx.value) sendTx.value = BigInt(tx.value);
  const chainId = typeof tx.chain_id === "bigint" ? Number(tx.chain_id) : tx.chain_id;
  if (chainId !== undefined) sendTx.chainId = chainId;

  const txHash = await signer.sendTransaction(sendTx);

  opts?.run?.attachTx(txHash, result.id);

  return { txHash, flightCheckId: result.id };
}
