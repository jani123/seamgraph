"""Read the Go demo's local event and print the shipping/reporting result."""
import json
import sys
from fulfillment.api import Worker
from reporting.summary import summary

worker = Worker()
shipment = worker.process(json.load(sys.stdin))
print(json.dumps(summary([shipment]), sort_keys=True))
