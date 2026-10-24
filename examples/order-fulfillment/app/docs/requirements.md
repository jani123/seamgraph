# Fulfillment requirements

- ORDER-1: retries of an identical order reserve and charge once; conflicting retries fail.
- ORDER-2: payment rejection releases the reservation.
- CANCEL-1: cancellation releases reserved stock and refunds once.
- SHIP-1: retries of order.ready create one shipment.

See [ADR 1](adr/0001-local-fakes.md). CANCEL-1 is implemented on the cancellation
branch; shipping cancellation is a tombstone, not physical parcel interception.
Contract mappings describe selected assertions, not complete requirement coverage.
