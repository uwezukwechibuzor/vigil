"""Example: guarded_send with web3.py."""

from vigil import VigilClient, FlightCheckRequest, guarded_send

vigil = VigilClient()

# Replace with actual web3 setup:
# from web3 import Web3
# w3 = Web3(Web3.HTTPProvider("https://base-rpc.example.com"))
# account = w3.eth.account.from_key(os.environ["PRIVATE_KEY"])
# send_fn = lambda tx: w3.eth.send_transaction({**tx, "from": account.address}).hex()

tx = FlightCheckRequest(
    **{
        "chain_id": 8453,
        "from": "0xYourAddress",
        "to": "0xContractAddress",
        "data": "0x",
        "value": "0x0",
        "intent": "swap USDC for ETH",
    }
)

# result = guarded_send(vigil, send_fn, tx, on_warn=lambda r: print("warn:", r.score))
# print("tx_hash:", result["tx_hash"])
print("Example ready — configure web3 send_fn to execute")
