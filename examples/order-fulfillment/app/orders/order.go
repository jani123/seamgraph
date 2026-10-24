package orders

import (
	"errors"
	"example.test/fulfillment/inventory"
	"example.test/fulfillment/payments"
)

type Status string

const Pending Status = "pending"
const Fulfilled Status = "fulfilled"
const Cancelled Status = "cancelled"

type Order struct {
	ID       string
	SKU      string
	Quantity int
	Amount   int
	Status   Status
}
type Service struct {
	stock   *inventory.Stock
	payment *payments.Adapter
	orders  map[string]Order
}

func New(stock *inventory.Stock, payment *payments.Adapter) *Service {
	return &Service{stock: stock, payment: payment, orders: map[string]Order{}}
}
func (s *Service) Place(id, sku string, quantity, amount int) (Order, error) {
	if old, ok := s.orders[id]; ok {
		if old.SKU != sku || old.Quantity != quantity || old.Amount != amount {
			return Order{}, errors.New("order identity reused")
		}
		return old, nil
	}
	if err := s.stock.Reserve(id, sku, quantity); err != nil {
		return Order{}, err
	}
	if err := s.payment.Authorize(id, amount); err != nil {
		s.stock.Release(id)
		return Order{}, err
	}
	order := Order{ID: id, SKU: sku, Quantity: quantity, Amount: amount, Status: Pending}
	s.orders[id] = order
	return order, nil
}
func (s *Service) Get(id string) (Order, bool) { order, ok := s.orders[id]; return order, ok }
func (s *Service) Fulfill(id string) error {
	order, ok := s.orders[id]
	if !ok || order.Status != Pending {
		return errors.New("order is not pending")
	}
	order.Status = Fulfilled
	s.orders[id] = order
	return nil
}

// StockReader illustrates a named interface contract and structural satisfaction.
type StockReader interface{ Available(sku string) int }
