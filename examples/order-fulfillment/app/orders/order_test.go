package orders

import (
	"example.test/fulfillment/inventory"
	"example.test/fulfillment/payments"
	"testing"
)

func TestPlace(t *testing.T) {
	stock, payment := inventory.New(), payments.New()
	service := New(stock, payment)
	first, err := service.Place("o-1", "book", 2, 1200)
	if err != nil || first.Status != Pending {
		t.Fatalf("place: %v %v", first, err)
	}
	if _, err := service.Place("o-1", "book", 2, 1200); err != nil {
		t.Fatal(err)
	}
	if stock.Available("book") != 2 || payment.Charges != 1 {
		t.Fatal("retry reserved or charged twice")
	}
	if _, err := service.Place("o-1", "pen", 1, 100); err == nil {
		t.Fatal("accepted conflicting retry")
	}
	if err := service.Fulfill("o-1"); err != nil {
		t.Fatal(err)
	}
	if err := service.Fulfill("o-1"); err == nil {
		t.Fatal("fulfilled twice")
	}
}
func TestAuthorizationFailure(t *testing.T) {
	stock := inventory.New()
	service := New(stock, payments.New())
	if _, err := service.Place("o-2", "book", 2, 0); err == nil {
		t.Fatal("expected payment failure")
	}
	if stock.Available("book") != 4 {
		t.Fatal("stock leaked after payment failure")
	}
	if _, err := service.Place("o-3", "book", 8, 100); err == nil {
		t.Fatal("oversold stock")
	}
}
