from dataclasses import dataclass

SCHEMA_VERSION: int = 1

def _tracking(order_id):
    return "local-" + order_id

@dataclass(frozen=True)
class Shipment:
    order_id: str
    tracking: str

class Worker:
    """In-memory shipping fake; callers supply decoded order.ready messages."""
    def __init__(self):
        self.shipments = {}

    def process(self, message):
        if message["type"] != "order.ready":
            raise ValueError("unsupported message")
        order_id = message["order_id"]
        if order_id not in self.shipments:
            self.shipments[order_id] = Shipment(order_id, _tracking(order_id))
        return self.shipments[order_id]

    @property
    def count(self):
        return len(self.shipments)
