package main

import (
	"encoding/json"
	"os"

	"example.test/fulfillment/inventory"
	"example.test/fulfillment/orders"
	"example.test/fulfillment/payments"
)

type ready struct {
	Type    string `json:"type"`
	OrderID string `json:"order_id"`
}

func main() {
	service := orders.New(inventory.New(), payments.New())
	order, err := service.Place("o-1", "book", 2, 1200)
	if err != nil {
		panic(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(ready{Type: "order.ready", OrderID: order.ID}); err != nil {
		panic(err)
	}
	if err := service.Fulfill(order.ID); err != nil {
		panic(err)
	}
}
