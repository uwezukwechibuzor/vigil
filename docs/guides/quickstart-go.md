---
title: "Quickstart — Go SDK"
description: "Run your first flight check from Go."
---

# Quickstart — Go SDK

## Install

```bash
go get github.com/uwezukwechibuzor/vigil/sdks/go
```

## Run a flight check

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func main() {
    client := vigil.NewClient(os.Getenv("VIGIL_API_KEY"))

    result, err := client.FlightCheck(context.Background(), vigil.FlightCheckRequest{
        Chain:  "ethereum",
        RPCURL: os.Getenv("ETH_RPC_URL"),
        TX: vigil.TX{
            From:  "0xYourWalletAddress",
            To:    "0xContractAddress",
            Value: "0xDE0B6B3A7640000",
            Data:  "0x",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    if result.RiskScore > 0.7 {
        log.Fatalf("high risk: %v", result.Checks)
    }

    fmt.Printf("verdict=%s risk_score=%.2f\n", result.Verdict, result.RiskScore)
}
```

## Disable telemetry

```bash
VIGIL_TELEMETRY=0 ./your-agent
```

## Next steps

- [Flight check reference](../reference/flight-check.md)
- [Self-hosting guide](../guides/self-hosting.md)
