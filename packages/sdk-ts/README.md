# @vigil/sdk-ts

TypeScript SDK for the Vigil reliability layer.

## Quickstart

```ts
import { VigilClient } from "@vigil/sdk-ts";

const vigil = new VigilClient({ apiKey: process.env.VIGIL_API_KEY });
const result = await vigil.flightCheck({ chain_id: 8453, from: "0x...", to: "0x...", data: "0x", value: "0x0" });
console.log(result.verdict); // "pass" | "warn" | "fail"
```
