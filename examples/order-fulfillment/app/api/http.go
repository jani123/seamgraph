// Package api provides a local, single-process HTTP fake. It is not concurrency safe.
package api

import (
	"encoding/json"
	"example.test/fulfillment/orders"
	"net/http"
)

type Request struct {
	ID       string `json:"id"`
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
	Amount   int    `json:"amount"`
}
type Response struct {
	ID     string        `json:"id"`
	Status orders.Status `json:"status"`
}

func Handler(service *orders.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			order, ok := service.Get(r.URL.Query().Get("id"))
			if !ok {
				http.Error(w, "missing order", http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(Response{ID: order.ID, Status: order.Status})
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		order, err := service.Place(request.ID, request.SKU, request.Quantity, request.Amount)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Response{ID: order.ID, Status: order.Status})
	})
}
