# vigil-go

Go SDK for the Vigil reliability layer.

## Quickstart

```go
import vigil "github.com/uwezukwechibuzor/vigil/sdks/go"

client := vigil.New() // reads VIGIL_API_KEY from env
result, err := client.FlightCheck(ctx, vigil.FlightCheckRequest{ChainID: 8453, ...})
```
