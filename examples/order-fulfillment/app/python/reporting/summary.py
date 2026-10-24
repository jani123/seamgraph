from fulfillment.api import Shipment

def summary(shipments: list[Shipment]) -> dict:
    return {"fulfilled": len(shipments), "order_ids": sorted(item.order_id for item in shipments)}

__all__ = ["summary"]
