package timeaction

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Order is one buyer purchase order being planned.
type Order struct {
	ID       string `json:"id"`
	Buyer    string `json:"buyer"`
	PONumber string `json:"poNumber"`
	Style    string `json:"style"`
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`

	// Chain names the milestone chain this order is planned on, so that one
	// buyer's longer approval loop does not change anybody else's plan.
	Chain string `json:"chain"`

	PODate   Date `json:"poDate"`
	ShipDate Date `json:"shipDate"`

	// Actuals holds the date each milestone actually finished on, by milestone
	// code. A milestone absent from this map has not been completed.
	Actuals map[string]Date `json:"actuals"`

	Note string `json:"note"`
}

// Normalise tidies an order.
func (o Order) Normalise() Order {
	o.ID = strings.TrimSpace(o.ID)
	o.Buyer = strings.TrimSpace(o.Buyer)
	o.PONumber = strings.TrimSpace(o.PONumber)
	o.Style = strings.TrimSpace(o.Style)
	o.Product = strings.TrimSpace(o.Product)
	o.Chain = strings.TrimSpace(o.Chain)
	o.Note = strings.TrimSpace(o.Note)
	if o.Quantity < 0 {
		o.Quantity = 0
	}

	actuals := map[string]Date{}
	for code, on := range o.Actuals {
		code = NormaliseCode(code)
		if code == "" || on.IsZero() {
			continue
		}
		actuals[code] = on
	}
	o.Actuals = actuals
	return o
}

// Validate reports every problem with an order at once.
func (o Order) Validate() error {
	var problems []string
	if o.Buyer == "" && o.PONumber == "" {
		problems = append(problems, "an order needs a buyer or a purchase-order number, so that it can be told apart from the others")
	}
	if o.ShipDate.IsZero() {
		problems = append(problems, "an order needs a ship date: that is the date the whole plan is worked backwards from")
	}
	if !o.PODate.IsZero() && !o.ShipDate.IsZero() && o.ShipDate.Before(o.PODate) {
		problems = append(problems, fmt.Sprintf("the ship date %s falls before the purchase-order date %s", o.ShipDate, o.PODate))
	}
	if o.Quantity < 0 {
		problems = append(problems, "a quantity cannot be below zero")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// Label is how the order reads on a board: the buyer, the style and the
// purchase-order number, in whatever combination the user has filled in.
func (o Order) Label() string {
	var parts []string
	if o.Buyer != "" {
		parts = append(parts, o.Buyer)
	}
	if o.Style != "" {
		parts = append(parts, o.Style)
	}
	if o.PONumber != "" {
		parts = append(parts, "PO "+o.PONumber)
	}
	if len(parts) == 0 {
		return "Untitled order"
	}
	return strings.Join(parts, " · ")
}

// WithActual records the date a milestone actually finished on, returning a new
// order and leaving the old one untouched.
func (o Order) WithActual(code string, on Date) (Order, error) {
	code = NormaliseCode(code)
	if code == "" {
		return o, errors.New("which milestone? a completion needs a milestone code")
	}
	if on.IsZero() {
		return o, errors.New("a completion needs the date the work actually finished on")
	}

	next := o.clone()
	next.Actuals[code] = on
	return next, nil
}

// WithoutActual clears a recorded completion.
func (o Order) WithoutActual(code string) (Order, error) {
	code = NormaliseCode(code)
	if _, ok := o.Actuals[code]; !ok {
		return o, fmt.Errorf("nothing is recorded as completed against %s", code)
	}
	next := o.clone()
	delete(next.Actuals, code)
	return next, nil
}

// clone copies an order, including its completions, so that one caller's edit
// cannot reach into another's copy.
func (o Order) clone() Order {
	next := o
	next.Actuals = map[string]Date{}
	for code, on := range o.Actuals {
		next.Actuals[code] = on
	}
	return next
}

// ActualList returns the recorded completions in a stable order, for printing
// and for tests.
func (o Order) ActualList() []Completion {
	out := make([]Completion, 0, len(o.Actuals))
	for code, on := range o.Actuals {
		out = append(out, Completion{Code: code, On: on})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].On != out[j].On {
			return out[i].On.Before(out[j].On)
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// Completion is one milestone's recorded finish date.
type Completion struct {
	Code string `json:"code"`
	On   Date   `json:"on"`
}
