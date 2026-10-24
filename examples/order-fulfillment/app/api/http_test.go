package api

import (
	"encoding/json"
	"example.test/fulfillment/inventory"
	"example.test/fulfillment/orders"
	"example.test/fulfillment/payments"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTP(t *testing.T) {
	handler := Handler(orders.New(inventory.New(), payments.New()))
	for _, test := range []struct {
		method, url, body string
		code              int
	}{
		{"POST", "/orders", `{"id":"o-1","sku":"book","quantity":2,"amount":1200}`, 201},
		{"GET", "/orders?id=o-1", "", 200},
		{"GET", "/orders?id=missing", "", 404},
		{"POST", "/orders", "{", 400},
		{"POST", "/orders", `{"id":"o-2","sku":"book","quantity":99,"amount":1200}`, 409},
		{"DELETE", "/orders", "", 405},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.url, strings.NewReader(test.body)))
		if recorder.Code != test.code {
			t.Fatalf("%s %s: %d %s", test.method, test.url, recorder.Code, recorder.Body.String())
		}
		if recorder.Code == http.StatusOK || recorder.Code == http.StatusCreated {
			var response Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.ID != "o-1" || response.Status != orders.Pending {
				t.Fatalf("response: %v %v", response, err)
			}
		}
	}
}
