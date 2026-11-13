# vigil-sdk

Python SDK for the Vigil reliability layer.

## Quickstart

```python
from vigil import VigilClient

vigil = VigilClient()  # reads VIGIL_API_KEY from env
result = vigil.flight_check(chain_id=8453, from_="0x...", to="0x...", data="0x", value="0x0")
print(result.verdict)  # "pass" | "warn" | "fail"
```
