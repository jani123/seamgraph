import unittest
from fulfillment.api import Worker
from operations.run import run

class FlowTest(unittest.TestCase):
    def test_shipping_retry(self):
        worker = Worker()
        first = worker.process({"type": "order.ready", "order_id": "o-1"})
        self.assertEqual(first, worker.process({"type": "order.ready", "order_id": "o-1"}))
        self.assertEqual(worker.count, 1)
        with self.assertRaises(ValueError):
            worker.process({"type": "unknown", "order_id": "o-1"})
        self.assertEqual(run(), {"fulfilled": 1, "order_ids": ["o-1"]})
