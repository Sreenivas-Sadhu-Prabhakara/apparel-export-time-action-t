package timeaction

import (
	"strings"
	"testing"
	"time"
)

func TestPlanCSVRows(t *testing.T) {
	p := tinyPlan(t, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 24), Settings: DefaultSettings(),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.June, 26)},
	})

	header := PlanCSVHeader()
	rows := PlanCSVRows(p)
	if len(rows) != len(p.Milestones) {
		t.Fatalf("wrote %d rows for %d milestones", len(rows), len(p.Milestones))
	}
	for i, row := range rows {
		if len(row) != len(header) {
			t.Fatalf("row %d has %d columns, the header has %d", i, len(row), len(header))
		}
	}

	// Cutting: done, two columns of dates, one day past its date.
	cut := rows[0]
	if cut[1] != "CUT" {
		t.Fatalf("first row is %q, want CUT", cut[1])
	}
	if cut[0] != "Cut" || cut[2] != "Cutting room" {
		t.Errorf("name and owner = %q and %q, want Cut and Cutting room", cut[0], cut[2])
	}
	if cut[3] != "2" || cut[4] != "working days" {
		t.Errorf("lead time = %q %q, want 2 working days", cut[3], cut[4])
	}
	if cut[5] != "2026-06-24" || cut[6] != "2026-06-25" {
		t.Errorf("planned span = %q..%q, want 2026-06-24..2026-06-25", cut[5], cut[6])
	}
	if cut[7] != "2026-06-26" {
		t.Errorf("actual finish = %q, want 2026-06-26", cut[7])
	}
	if cut[12] != "Done" || cut[13] != "1" {
		t.Errorf("status and days late = %q and %q, want Done and 1", cut[12], cut[13])
	}
	if cut[11] != "yes" {
		t.Errorf("critical path = %q, want yes", cut[11])
	}

	// Sewing: not done, so the days-late column carries the projected slip and
	// the waiting-on column names what is holding it up.
	sew := rows[1]
	if sew[12] != "Late" || sew[13] != "1" {
		t.Errorf("sewing status and slip = %q and %q, want Late and 1", sew[12], sew[13])
	}
	if sew[15] != "CUT" {
		t.Errorf("sewing is waiting on %q, want CUT", sew[15])
	}
	// The on-board milestone is counted in calendar days.
	if ship := rows[2]; ship[4] != "calendar days" {
		t.Errorf("the on-board row is counted in %q, want calendar days", ship[4])
	}
}

func TestPlanCSVForAnEmptyPlan(t *testing.T) {
	if rows := PlanCSVRows(Plan{}); len(rows) != 0 {
		t.Errorf("wrote %d rows for a plan with no milestones", len(rows))
	}
}

func TestBoardCSVRows(t *testing.T) {
	asOf := NewDate(2026, time.June, 24)
	row := boardCase(t, "Northbridge Apparel", 2400, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: asOf, Settings: DefaultSettings(),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.June, 26)},
	})
	row.Order.PONumber = "4471"
	row.Order.Style = "LS-TEE-220"
	board := BuildBoard(asOf, []Row{row})

	header := BoardCSVHeader()
	rows := BoardCSVRows(board)
	if len(rows) != 1 {
		t.Fatalf("wrote %d rows, want 1", len(rows))
	}
	if len(rows[0]) != len(header) {
		t.Fatalf("the row has %d columns, the header has %d", len(rows[0]), len(header))
	}

	got := rows[0]
	if got[0] != "Northbridge Apparel" || got[1] != "4471" || got[2] != "LS-TEE-220" {
		t.Errorf("identity columns = %q, %q, %q", got[0], got[1], got[2])
	}
	if got[3] != "2400" {
		t.Errorf("quantity = %q, want 2400", got[3])
	}
	if got[6] != "2026-06-30" {
		t.Errorf("ship date = %q, want 2026-06-30", got[6])
	}
	if got[7] != "6" {
		t.Errorf("days to ship = %q, want 6", got[7])
	}
	if got[8] != "Late" {
		t.Errorf("status = %q, want Late", got[8])
	}
	if got[9] != "2026-07-01" || got[10] != "1" {
		t.Errorf("projected on board %q, %q days late; want 2026-07-01 and 1", got[9], got[10])
	}
	if got[13] != "Sew" {
		t.Errorf("chase = %q, want Sew", got[13])
	}
	if got[16] != "1 of 3" {
		t.Errorf("milestones done = %q, want \"1 of 3\"", got[16])
	}
	if !strings.Contains(got[18], "Running late") {
		t.Errorf("verdict column = %q", got[18])
	}
}

func TestBoardCSVForAnEmptyBoard(t *testing.T) {
	if rows := BoardCSVRows(Board{}); len(rows) != 0 {
		t.Errorf("wrote %d rows for an empty board", len(rows))
	}
}

func TestYesNo(t *testing.T) {
	if got := yesNo(true); got != "yes" {
		t.Errorf("yesNo(true) = %q", got)
	}
	if got := yesNo(false); got != "no" {
		t.Errorf("yesNo(false) = %q", got)
	}
}
