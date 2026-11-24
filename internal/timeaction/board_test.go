package timeaction

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// boardCase builds one order and its plan on the tiny chain.
func boardCase(t *testing.T, buyer string, quantity int, in Input) Row {
	t.Helper()
	order := Order{Buyer: buyer, Quantity: quantity, Chain: "Tiny", ShipDate: in.ShipDate, PODate: in.PODate, Actuals: in.Actuals}.Normalise()
	plan := tinyPlan(t, in)
	return BoardRow(order, plan)
}

func TestBuildBoardCountsAndSortsWorstFirst(t *testing.T) {
	asOf := NewDate(2026, time.June, 24)

	// Late: cutting finished a day past its date, which moves the ship date.
	late := boardCase(t, "Late buyer", 3000, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings(),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.June, 26)},
	})
	// At risk: three days of slack against the shipped threshold.
	atRisk := boardCase(t, "Tight buyer", 1200, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings(),
	})
	// On track: placed early, so there is real cushion.
	onTrack := boardCase(t, "Early buyer", 800, Input{
		PODate: NewDate(2026, time.June, 1), AsOf: asOf, Settings: DefaultSettings(),
	})
	// Complete: everything recorded.
	complete := boardCase(t, "Shipped buyer", 500, Input{
		PODate: NewDate(2026, time.June, 1), AsOf: asOf, Settings: DefaultSettings(),
		Actuals: map[string]Date{
			"CUT":  NewDate(2026, time.June, 10),
			"SEW":  NewDate(2026, time.June, 15),
			"SHIP": NewDate(2026, time.June, 16),
		},
	})

	board := BuildBoard(asOf, []Row{complete, onTrack, atRisk, late})

	if board.Late != 1 || board.AtRisk != 1 || board.OnTrack != 1 || board.Complete != 1 {
		t.Errorf("counts: late %d, at risk %d, on track %d, complete %d; want one of each",
			board.Late, board.AtRisk, board.OnTrack, board.Complete)
	}
	if board.Unusable != 0 {
		t.Errorf("unusable = %d, want 0", board.Unusable)
	}
	if want := 5500; board.Pieces != want {
		t.Errorf("pieces = %d, want %d", board.Pieces, want)
	}
	// Only the late and at-risk orders count as pieces at risk.
	if want := 4200; board.PiecesAtRisk != want {
		t.Errorf("pieces at risk = %d, want %d", board.PiecesAtRisk, want)
	}

	want := []string{"Late buyer", "Tight buyer", "Early buyer", "Shipped buyer"}
	for i, buyer := range want {
		if got := board.Rows[i].Order.Buyer; got != buyer {
			t.Errorf("row %d is %q, want %q (full order: %v)", i, got, buyer, buyerNames(board))
		}
	}
}

func TestBuildBoardBreaksTiesByTheWorstFigures(t *testing.T) {
	asOf := NewDate(2026, time.June, 24)

	// Both late, one by more days.
	oneDay := boardCase(t, "One day late", 100, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings(),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.June, 26)},
	})
	manyDays := boardCase(t, "Badly late", 100, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings(),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.July, 6)},
	})
	if manyDays.SlipDays <= oneDay.SlipDays {
		t.Fatalf("the badly late order slips %d days and the other %d; the fixture is wrong",
			manyDays.SlipDays, oneDay.SlipDays)
	}

	board := BuildBoard(asOf, []Row{oneDay, manyDays})
	if got := board.Rows[0].Order.Buyer; got != "Badly late" {
		t.Errorf("first row = %q, want the order that is furthest behind", got)
	}

	// Neither late, one with less slack.
	tight := boardCase(t, "Tight", 100, Input{PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings()})
	roomy := boardCase(t, "Roomy", 100, Input{PODate: NewDate(2026, time.June, 1), AsOf: asOf, Settings: DefaultSettings()})
	board = BuildBoard(asOf, []Row{roomy, tight})
	if got := board.Rows[0].Order.Buyer; got != "Tight" {
		t.Errorf("first row = %q, want the order with the least slack", got)
	}

	// Same status and same slack: the one shipping soonest comes first. The two
	// fixtures are four weeks apart so they fall on the same weekdays and carry
	// the same slack.
	soon := boardCase(t, "Soon", 100, Input{
		ShipDate: NewDate(2026, time.June, 30), PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings()})
	later := boardCase(t, "Later", 100, Input{
		ShipDate: NewDate(2026, time.July, 28), PODate: NewDate(2026, time.July, 20), AsOf: asOf, Settings: DefaultSettings()})
	if soon.MinFloatDays != later.MinFloatDays {
		t.Fatalf("the fixture orders have %d and %d days of slack; they should match", soon.MinFloatDays, later.MinFloatDays)
	}
	board = BuildBoard(asOf, []Row{later, soon})
	if got := board.Rows[0].Order.Buyer; got != "Soon" {
		t.Errorf("first row = %q, want the order shipping soonest", got)
	}
}

func TestBoardRowCarriesTheMilestoneToChaseAndTheOverdueCount(t *testing.T) {
	asOf := NewDate(2026, time.June, 29)
	row := boardCase(t, "Northbridge Apparel", 2400, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings(),
	})

	if row.ChaseCode != "CUT" {
		t.Errorf("chase = %q, want CUT", row.ChaseCode)
	}
	if row.ChaseName != "Cut" || row.ChaseOwner != "Cutting room" {
		t.Errorf("chase = %q owned by %q, want the milestone's name and owner", row.ChaseName, row.ChaseOwner)
	}
	if want := NewDate(2026, time.June, 25); row.ChaseDue != want {
		t.Errorf("chase due %v, want %v", row.ChaseDue, want)
	}
	// Cutting's date, 25 June, is behind us with nothing recorded. Sewing is
	// due today, 29 June, and a milestone due today is not yet overdue.
	if want := 1; row.Overdue != want {
		t.Errorf("overdue milestones = %d, want %d", row.Overdue, want)
	}
	if row.Label != "Northbridge Apparel" {
		t.Errorf("label = %q, want the buyer", row.Label)
	}
	if row.DoneCount != 0 || row.TotalCount != 3 {
		t.Errorf("done %d of %d, want 0 of 3", row.DoneCount, row.TotalCount)
	}
	if row.Verdict == "" {
		t.Error("the row carries no verdict")
	}
}

func TestAnOrderThatCannotBePlannedStaysOnTheBoard(t *testing.T) {
	broken := Order{Buyer: "Broken buyer", Quantity: 700}.Normalise()
	row := FailedRow(broken, errors.New("the chain has a circle in it"))

	board := BuildBoard(NewDate(2026, time.June, 24), []Row{row})
	if board.Unusable != 1 {
		t.Errorf("unusable = %d, want 1", board.Unusable)
	}
	if len(board.Rows) != 1 {
		t.Fatalf("the board has %d rows, want the broken order still on it", len(board.Rows))
	}
	if !strings.Contains(board.Rows[0].Verdict, "circle") {
		t.Errorf("verdict = %q, want it to carry the reason", board.Rows[0].Verdict)
	}
	// A broken order is not counted as at risk, because nothing is known about it.
	if board.PiecesAtRisk != 0 {
		t.Errorf("pieces at risk = %d, want 0", board.PiecesAtRisk)
	}
}

func TestBuildBoardWithNoOrders(t *testing.T) {
	board := BuildBoard(Date{}, nil)
	if board.AsOf.IsZero() {
		t.Error("an empty board has no date on it")
	}
	if len(board.Rows) != 0 || board.Pieces != 0 {
		t.Errorf("an empty board reports %d rows and %d pieces", len(board.Rows), board.Pieces)
	}
}

func buyerNames(b Board) []string {
	out := make([]string, 0, len(b.Rows))
	for _, r := range b.Rows {
		out = append(out, r.Order.Buyer)
	}
	return out
}
