package inventory

import "errors"

// ErrStock means the requested units cannot be reserved.
var ErrStock = errors.New("insufficient stock")

type Reservation struct {
	SKU      string
	Quantity int
}
type Stock struct {
	available    map[string]int
	reservations map[string]Reservation
}

func New() *Stock { return &Stock{map[string]int{"book": 4, "pen": 10}, map[string]Reservation{}} }
func (s *Stock) Reserve(id, sku string, quantity int) error {
	if previous, ok := s.reservations[id]; ok {
		if previous.SKU == sku && previous.Quantity == quantity {
			return nil
		}
		return errors.New("reservation identity reused")
	}
	if quantity <= 0 || s.available[sku] < quantity {
		return ErrStock
	}
	s.available[sku] -= quantity
	s.reservations[id] = Reservation{sku, quantity}
	return nil
}
func (s *Stock) Release(id string) {
	if reservation, ok := s.reservations[id]; ok {
		s.available[reservation.SKU] += reservation.Quantity
		delete(s.reservations, id)
	}
}
func (s *Stock) Available(sku string) int { return s.available[sku] }

// DebugAvailable is deliberately outside the public seam.
func DebugAvailable(s *Stock, sku string) int { return s.available[sku] }
