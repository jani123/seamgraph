from fulfillment.api import Worker
from reporting.summary import summary

def run():
    worker = Worker()
    shipment = worker.process({"type": "order.ready", "order_id": "o-1"})
    return summary([shipment])
