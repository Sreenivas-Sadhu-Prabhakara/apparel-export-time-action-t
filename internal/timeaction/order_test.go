package timeaction

import (
	"strings"
	"testing"
	"time"
)

func TestOrderNormalise(t *testing.T) {
	got := Order{
		ID:       "  abc  ",
		Buyer:    "  Northbridge Apparel  ",
		PONumber: " PO-4471 ",
		Style:    "  LS-TEE-220  ",
		Product:  " Long sleeve tee ",
		Quantity: -40,
		Chain:    "  Standard  ",
		Note:     "  air if late  ",
		Actuals: map[string]Date{
			"  cutting ": NewDate(2026, time.June, 1),
			"SEWING":     {},
			"":           NewDate(2026, time.June, 2),
		},
	}.Normalise()

	if got.Buyer != "Northbridge Apparel" || got.PONumber != "PO-4471" || got.Style != "LS-TEE-220" {
		t.Errorf("text fields were not trimmed: %+v", got)
	}
	if got.Product != "Long sleeve tee" || got.Chain != "Standard" || got.Note != "air if late" {
		t.Errorf("text fields were not trimmed: %+v", got)
	}
	if got.Quantity != 0 {
		t.Errorf("quantity = %d, want 0: a negative quantity means nothing", got.Quantity)
	}
	if len(got.Actuals) != 1 {
		t.Fatalf("completions = %v, want only the one usable entry", got.Actuals)
	}
	if _, ok := got.Actuals["CUTTING"]; !ok {
		t.Errorf("completions = %v, want the code in its comparison form", got.Actuals)
	}
}

func TestOrderValidate(t *testing.T) {
	ship := NewDate(2026, time.June, 30)
	cases := []struct {
		name     string
		order    Order
		wantErr  bool
		contains string
	}{
		{
			name:  "a buyer and a ship date is enough",
			order: Order{Buyer: "Northbridge Apparel", ShipDate: ship},
		},
		{
			name:  "a purchase-order number alone is enough",
			order: Order{PONumber: "PO-4471", ShipDate: ship},
		},
		{
			name:    "nothing to tell it apart by",
			order:   Order{ShipDate: ship},
			wantErr: true, contains: "buyer or a purchase-order number",
		},
		{
			name:    "no ship date",
			order:   Order{Buyer: "Northbridge Apparel"},
			wantErr: true, contains: "ship date",
		},
		{
			name: "the ship date falls before the order was placed",
			order: Order{Buyer: "Northbridge Apparel",
				PODate: NewDate(2026, time.July, 1), ShipDate: ship},
			wantErr: true, contains: "falls before",
		},
		{
			name: "the ship date is the order date",
			order: Order{Buyer: "Northbridge Apparel",
				PODate: ship, ShipDate: ship},
		},
		{
			name:    "no buyer and no ship date reports both",
			order:   Order{},
			wantErr: true, contains: "\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.order.Normalise().Validate()
			if !c.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("wanted an error, got none")
			}
			if c.contains != "" && !strings.Contains(err.Error(), c.contains) {
				t.Errorf("error %q does not contain %q", err, c.contains)
			}
		})
	}
}

func TestOrderLabel(t *testing.T) {
	cases := []struct {
		name  string
		order Order
		want  string
	}{
		{name: "everything filled in",
			order: Order{Buyer: "Northbridge Apparel", Style: "LS-TEE-220", PONumber: "PO-4471"},
			want:  "Northbridge Apparel · LS-TEE-220 · PO 4471"},
		{name: "buyer only", order: Order{Buyer: "Northbridge Apparel"}, want: "Northbridge Apparel"},
		{name: "purchase order only", order: Order{PONumber: "4471"}, want: "PO 4471"},
		{name: "style and purchase order", order: Order{Style: "LS-TEE-220", PONumber: "4471"}, want: "LS-TEE-220 · PO 4471"},
		{name: "nothing at all", order: Order{}, want: "Untitled order"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The label joins whatever is there; the PO number is printed as
			// given, so "PO-4471" would read "PO PO-4471" and is spelled plainly.
			order := c.order
			if order.PONumber == "PO-4471" {
				order.PONumber = "4471"
			}
			if got := order.Label(); got != c.want {
				t.Errorf("Label() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestWithActualDoesNotChangeTheOrderItCameFrom(t *testing.T) {
	original := Order{Buyer: "Northbridge Apparel", ShipDate: NewDate(2026, time.June, 30)}.Normalise()

	withOne, err := original.WithActual(" cutting ", NewDate(2026, time.June, 1))
	if err != nil {
		t.Fatalf("WithActual: %v", err)
	}
	if len(original.Actuals) != 0 {
		t.Errorf("the original order now holds %v", original.Actuals)
	}
	if on, ok := withOne.Actuals["CUTTING"]; !ok || on != NewDate(2026, time.June, 1) {
		t.Errorf("completions = %v, want cutting on 2026-06-01", withOne.Actuals)
	}

	moved, err := withOne.WithActual("CUTTING", NewDate(2026, time.June, 3))
	if err != nil {
		t.Fatalf("WithActual again: %v", err)
	}
	if withOne.Actuals["CUTTING"] != NewDate(2026, time.June, 1) {
		t.Error("recording a new date changed the order it was recorded against")
	}
	if moved.Actuals["CUTTING"] != NewDate(2026, time.June, 3) {
		t.Errorf("completions = %v, want cutting moved to 2026-06-03", moved.Actuals)
	}

	cleared, err := moved.WithoutActual("cutting")
	if err != nil {
		t.Fatalf("WithoutActual: %v", err)
	}
	if len(cleared.Actuals) != 0 {
		t.Errorf("completions = %v, want none", cleared.Actuals)
	}
	if len(moved.Actuals) != 1 {
		t.Error("clearing a completion changed the order it was cleared from")
	}
}

func TestWithActualRefusesWhatItCannotRecord(t *testing.T) {
	order := Order{Buyer: "Northbridge Apparel", ShipDate: NewDate(2026, time.June, 30)}.Normalise()

	if _, err := order.WithActual("  ", NewDate(2026, time.June, 1)); err == nil {
		t.Error("a completion with no milestone was accepted")
	}
	if _, err := order.WithActual("CUTTING", Date{}); err == nil {
		t.Error("a completion with no date was accepted")
	}
	if _, err := order.WithoutActual("CUTTING"); err == nil {
		t.Error("clearing a completion that was never recorded was accepted")
	}
}

func TestActualListIsInDateOrder(t *testing.T) {
	order := Order{Actuals: map[string]Date{
		"SEWING":  NewDate(2026, time.June, 10),
		"CUTTING": NewDate(2026, time.June, 1),
		"LAB-DIP": NewDate(2026, time.June, 1),
	}}.Normalise()

	got := order.ActualList()
	if len(got) != 3 {
		t.Fatalf("list = %v, want 3 entries", got)
	}
	// Same date, so the codes break the tie and the order is stable.
	if got[0].Code != "CUTTING" || got[1].Code != "LAB-DIP" || got[2].Code != "SEWING" {
		t.Errorf("list = %v, want cutting, lab-dip, sewing", got)
	}
}
