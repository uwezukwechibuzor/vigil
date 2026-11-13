// Example: guardedSend with viem on Base Sepolia
import { VigilClient, guardedSend, viemSigner } from "../src/index.js";

// In a real app you'd import createWalletClient from "viem" and configure it.
// Here we show the pattern:
const vigil = new VigilClient(); // reads VIGIL_API_KEY from env

// Placeholder: replace with actual viem WalletClient
const walletClient = {
  sendTransaction: async (tx: unknown) => {
    console.log("Would send tx:", tx);
    return "0xtxhash" as `0x${string}`;
  },
};

const { txHash, flightCheckId } = await guardedSend(
  vigil,
  viemSigner(walletClient),
  {
    chain_id: 84532, // Base Sepolia
    from: "0xYourAddress",
    to: "0xContractAddress",
    data: "0x",
    value: "0x0",
    intent: "swap USDC for ETH",
  },
  {
    onWarn: async (result) => {
      console.warn("Flight check warn:", result.score, result.checks);
    },
  },
);

console.log("tx sent:", txHash, "flight check:", flightCheckId);
