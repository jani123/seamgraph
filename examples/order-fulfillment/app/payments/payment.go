package payments

import "errors"

type Authorization struct {
	OrderID  string
	Amount   int
	Refunded bool
}
type Adapter struct {
	records map[string]Authorization
	Charges int
	Refunds int
}

func New() *Adapter { return &Adapter{records: map[string]Authorization{}} }
func (a *Adapter) Authorize(id string, amount int) error {
	if amount <= 0 {
		return errors.New("invalid amount")
	}
	if old, ok := a.records[id]; ok {
		if old.Amount != amount || old.Refunded {
			return errors.New("authorization identity reused")
		}
		return nil
	}
	a.records[id] = Authorization{OrderID: id, Amount: amount}
	a.Charges++
	return nil
}
func (a *Adapter) Refund(id string) {
	record, ok := a.records[id]
	if !ok || record.Refunded {
		return
	}
	record.Refunded = true
	a.records[id] = record
	a.Refunds++
}
