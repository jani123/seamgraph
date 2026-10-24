# Wire contracts v1

POST /orders: {id, sku, quantity, amount}; amount is positive integer cents.
201: {id, status}; invalid JSON is 400, reservation/payment conflict is 409.
GET /orders?id=o-1 returns {id, status}, or 404. This bounded fake routes all paths
through the same handler; it is designed for in-process httptest, not public hosting.

order.ready v1: {type: "order.ready", order_id: "o-1"}. Worker returns a Shipment.
On the cancellation branch: order.cancelled v1: {type: "order.cancelled", order_id}.
The worker records a tombstone and refuses subsequent ready messages for that ID.
There is no broker: tests pass decoded dictionaries directly. These documents
express integration intent; SeamGraph does not infer Go-to-Python or TS-to-Go edges.
