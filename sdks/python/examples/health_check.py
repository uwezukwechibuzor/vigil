"""Example: check Base chain health."""

from vigil import VigilClient

vigil = VigilClient()  # reads VIGIL_API_KEY from env
health = vigil.chain_health(8453)
print(f"Base health: {health.status} | fee: {health.fee_level}")
