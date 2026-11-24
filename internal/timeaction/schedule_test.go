package timeaction

import (
	"strings"
	"testing"
	"time"
)

// tinyChain is a three-milestone chain small enough to work out by hand, so
// that the status and knock-on rules are tested against arithmetic a reader can
// check rather than against the shipped sixteen-milestone chain.
//
//	CUT   2 working days
//	SEW   3 working days, after CUT
//	SHIP  1 calendar day, after SEW
func tinyChain() Chain {
	return Chain{Name: "Tiny", Stages: []Stage{
		{Code: "CUT", Name: "Cut", Owner: "Cutting room", Days: 2, Basis: WorkingDays},
		{Code: "SEW", Name: "Sew", Owner: "Production", Days: 3, Basis: WorkingDays, After: []string{"CUT"}},
		{Code: "SHIP", Name: "On board", Owner: "Carrier", Days: 1, Basis: CalendarDays, After: []string{"SEW"}},
	}}
}

// june2026 is the ship date every tiny-chain case is scheduled backwards from.
// Tuesday 30 June 2026, with Sunday 28 June inside the sewing window.
var june2026 = NewDate(2026, time.June, 30)

func tinyPlan(t *testing.T, in Input) Plan {
	t.Helper()
	if in.Chain.Stages == nil {
		in.Chain = tinyChain()
	}
	if in.Calendar.WeeklyOff == nil {
		in.Calendar = sundayOff()
	}
	if in.ShipDate.IsZero() {
		in.ShipDate = june2026
	}
	p, err := Schedule(in)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	return p
}

func milestone(t *testing.T, p Plan, code string) Milestone {
	t.Helper()
	m, ok := p.Find(code)
	if !ok {
		t.Fatalf("the plan has no milestone %s", code)
	}
	return m
}

func TestScheduleBackwardFromTheShipDate(t *testing.T) {
	p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22)})

	cases := []struct {
		code                string
		wantStart, wantFini Date
	}{
		// On board is a calendar day and finishes on the ship date itself.
		{"SHIP", NewDate(2026, time.June, 30), NewDate(2026, time.June, 30)},
		// Sewing must finish the last working day before that: Monday 29 June.
		// Three working days back over Sunday 28 June is Friday 26 June.
		{"SEW", NewDate(2026, time.June, 26), NewDate(2026, time.June, 29)},
		// Cutting must finish the working day before sewing starts.
		{"CUT", NewDate(2026, time.June, 24), NewDate(2026, time.June, 25)},
	}

	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			m := milestone(t, p, c.code)
			if m.PlannedStart != c.wantStart {
				t.Errorf("planned start = %v, want %v", m.PlannedStart, c.wantStart)
			}
			if m.PlannedFinish != c.wantFini {
				t.Errorf("planned finish = %v, want %v", m.PlannedFinish, c.wantFini)
			}
		})
	}

	// The last day the order could have been placed is the planned start of the
	// first milestone.
	if want := NewDate(2026, time.June, 24); p.LatestPODate != want {
		t.Errorf("latest purchase-order date = %v, want %v", p.LatestPODate, want)
	}
}

func TestScheduleNeedsAShipDate(t *testing.T) {
	_, err := Schedule(Input{Chain: tinyChain(), Calendar: sundayOff()})
	if err == nil {
		t.Fatal("a plan with no ship date was accepted")
	}
	if !strings.Contains(err.Error(), "ship date") {
		t.Errorf("error %q does not say the ship date is missing", err)
	}
}

func TestScheduleRefusesABadChainAndABadCalendar(t *testing.T) {
	circular := Chain{Stages: []Stage{
		{Code: "A", Days: 1, Basis: WorkingDays, After: []string{"B"}},
		{Code: "B", Days: 1, Basis: WorkingDays, After: []string{"A"}},
	}}
	if _, err := Schedule(Input{Chain: circular, Calendar: sundayOff(), ShipDate: june2026}); err == nil {
		t.Error("a chain with a circle in it was scheduled")
	}

	shutForever := Calendar{WeeklyOff: []time.Weekday{
		time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}}
	if _, err := Schedule(Input{Chain: tinyChain(), Calendar: shutForever, ShipDate: june2026}); err == nil {
		t.Error("a calendar with no working day was scheduled")
	}
}

func TestScheduleForwardPassAndFloat(t *testing.T) {
	p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22)})

	cases := []struct {
		code                string
		wantStart, wantFini Date
		wantFloatCalendar   int
	}{
		{"CUT", NewDate(2026, time.June, 22), NewDate(2026, time.June, 23), 2},
		{"SEW", NewDate(2026, time.June, 24), NewDate(2026, time.June, 26), 2},
		{"SHIP", NewDate(2026, time.June, 27), NewDate(2026, time.June, 27), 3},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			m := milestone(t, p, c.code)
			if m.EarliestStart != c.wantStart {
				t.Errorf("earliest start = %v, want %v", m.EarliestStart, c.wantStart)
			}
			if m.EarliestFinish != c.wantFini {
				t.Errorf("earliest finish = %v, want %v", m.EarliestFinish, c.wantFini)
			}
			if m.FloatCalendarDays != c.wantFloatCalendar {
				t.Errorf("slack = %d calendar days, want %d", m.FloatCalendarDays, c.wantFloatCalendar)
			}
		})
	}

	if want := NewDate(2026, time.June, 27); p.EarliestShipDate != want {
		t.Errorf("earliest ship date = %v, want %v", p.EarliestShipDate, want)
	}
	if !p.Feasible || p.ShortfallDays != 0 {
		t.Errorf("feasible = %v, short by %d days; want it feasible and short by nothing", p.Feasible, p.ShortfallDays)
	}
	if want := 3; p.MinFloatDays != want {
		t.Errorf("the order's slack = %d days, want %d", p.MinFloatDays, want)
	}
}

func TestFloatIsReportedInTheMilestonesOwnUnit(t *testing.T) {
	p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22)})

	// Cutting can start on Monday 22 June at the earliest and Wednesday 24 June
	// at the latest: two calendar days of slack, and also two working days,
	// since no weekly off falls between them.
	cut := milestone(t, p, "CUT")
	if cut.FloatDays != 2 || cut.FloatCalendarDays != 2 {
		t.Errorf("cutting slack = %d working and %d calendar days, want 2 and 2", cut.FloatDays, cut.FloatCalendarDays)
	}

	// Sewing can start Wednesday 24 June at the earliest and Friday 26 June at
	// the latest. Two of each, again with no weekly off in between.
	sew := milestone(t, p, "SEW")
	if sew.FloatDays != 2 || sew.FloatCalendarDays != 2 {
		t.Errorf("sewing slack = %d working and %d calendar days, want 2 and 2", sew.FloatDays, sew.FloatCalendarDays)
	}

	// Going on board is a calendar milestone: Saturday 27 June at the earliest,
	// Tuesday 30 June at the latest, and a vessel does not keep a weekly off.
	ship := milestone(t, p, "SHIP")
	if ship.FloatDays != 3 || ship.FloatCalendarDays != 3 {
		t.Errorf("on-board slack = %d calendar days both ways, want 3", ship.FloatDays)
	}

	// A working-day milestone whose slack crosses the weekly off reports fewer
	// working days of slack than calendar days, because the weekly off is not
	// slack anybody can spend.
	wide := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 1), AsOf: NewDate(2026, time.June, 1)})
	cut = milestone(t, wide, "CUT")
	if cut.FloatDays >= cut.FloatCalendarDays {
		t.Errorf("cutting slack = %d working and %d calendar days; over three weeks the working figure must be the smaller",
			cut.FloatDays, cut.FloatCalendarDays)
	}
}

func TestCriticalPathIsAConnectedChainThroughTheWholeNetwork(t *testing.T) {
	ch := DefaultChain().Normalise()
	p, err := Schedule(Input{
		Chain: ch, Calendar: sundayOff(), Settings: DefaultSettings(),
		ShipDate: NewDate(2026, time.June, 30),
		PODate:   NewDate(2026, time.February, 2),
		AsOf:     NewDate(2026, time.February, 10),
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if len(p.CriticalPath) < 2 {
		t.Fatalf("critical path = %v, want a chain", p.CriticalPath)
	}

	// It must start at a milestone that waits on nothing and end at the one the
	// ship date hangs on.
	if got, want := p.CriticalPath[0], StagePO; got != want {
		t.Errorf("critical path starts at %s, want %s", got, want)
	}
	if got, want := p.CriticalPath[len(p.CriticalPath)-1], StageOnBoard; got != want {
		t.Errorf("critical path ends at %s, want %s", got, want)
	}

	// Every step of it must be a real dependency. Picking the milestones with
	// the least float instead of tracing the chain produced a disconnected set
	// here, because spans counted in working days pick up a day as they snap
	// over the weekly off.
	for i := 1; i < len(p.CriticalPath); i++ {
		prev, next := p.CriticalPath[i-1], p.CriticalPath[i]
		stage, ok := ch.Find(next)
		if !ok {
			t.Fatalf("%s is on the critical path but not in the chain", next)
		}
		linked := false
		for _, pred := range stage.After {
			if pred == prev {
				linked = true
				break
			}
		}
		if !linked {
			t.Errorf("%s follows %s on the critical path but does not wait on it", next, prev)
		}
	}

	// The shipped chain's long pole runs through fabric and sewing, and the
	// milestones that run beside it are not on the path.
	onPath := map[string]bool{}
	for _, code := range p.CriticalPath {
		onPath[code] = true
	}
	for _, code := range []string{StageFabricIn, StageSewing, StageCutting, StageFinishing} {
		if !onPath[code] {
			t.Errorf("%s is not on the critical path, and it should be", code)
		}
	}
	for _, code := range []string{StageTrimsOrder, StageTrimsIn, StageFitSample} {
		if onPath[code] {
			t.Errorf("%s is on the critical path, but it has slack beside the long pole", code)
		}
	}

	// Every milestone on the path is flagged, and nothing else is.
	for _, m := range p.Milestones {
		if m.Critical != onPath[m.Code] {
			t.Errorf("%s has critical = %v, want %v", m.Code, m.Critical, onPath[m.Code])
		}
	}
}

func TestAnOrderThatCannotBeMadeOnTimeSaysSoAndByHowMuch(t *testing.T) {
	// The shipped chain needs the order placed by 25 March 2026 for a 30 June
	// ship date. Placing it on 1 May leaves it unmakeable.
	p, err := Schedule(Input{
		Chain: DefaultChain(), Calendar: sundayOff(), Settings: DefaultSettings(),
		ShipDate: NewDate(2026, time.June, 30),
		PODate:   NewDate(2026, time.May, 1),
		AsOf:     NewDate(2026, time.May, 1),
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if p.Feasible {
		t.Fatal("an order with nine weeks for a fourteen-week chain was called feasible")
	}
	if want := NewDate(2026, time.August, 6); p.EarliestShipDate != want {
		t.Errorf("earliest ship date = %v, want %v", p.EarliestShipDate, want)
	}
	if want := 37; p.ShortfallDays != want {
		t.Errorf("short by %d days, want %d", p.ShortfallDays, want)
	}
	if want := -37; p.MinFloatDays != want {
		t.Errorf("the order's slack = %d days, want %d", p.MinFloatDays, want)
	}
	if p.Status != StatusLate {
		t.Errorf("status = %s, want %s", p.Status, StatusLate)
	}
	if !strings.Contains(p.Verdict, "Cannot be made on time") || !strings.Contains(p.Verdict, "37 days") {
		t.Errorf("verdict = %q, want it to say it cannot be made on time and by how much", p.Verdict)
	}
}

func TestAFeasibleOrderIsNotCalledShort(t *testing.T) {
	p, err := Schedule(Input{
		Chain: DefaultChain(), Calendar: sundayOff(), Settings: DefaultSettings(),
		ShipDate: NewDate(2026, time.June, 30),
		PODate:   NewDate(2026, time.February, 2),
		AsOf:     NewDate(2026, time.February, 10),
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if !p.Feasible || p.ShortfallDays != 0 {
		t.Errorf("feasible = %v, short by %d; want it feasible", p.Feasible, p.ShortfallDays)
	}
	if want := 51; p.MinFloatDays != want {
		t.Errorf("the order's slack = %d days, want %d", p.MinFloatDays, want)
	}
	if p.Status != StatusOnTrack {
		t.Errorf("status = %s, want %s", p.Status, StatusOnTrack)
	}
}

func TestHolidaysMoveTheDateTheOrderHadToBePlaced(t *testing.T) {
	// Five working days declared off inside the sewing window. Nothing about
	// the order changes; the plan simply has to start earlier. This is the slip
	// nobody sees on a plan kept in a spreadsheet.
	plain := sundayOff()
	shut := sundayOff(
		Holiday{On: NewDate(2026, time.June, 1), Name: "Factory holiday 1"},
		Holiday{On: NewDate(2026, time.June, 2), Name: "Factory holiday 2"},
		Holiday{On: NewDate(2026, time.June, 3), Name: "Factory holiday 3"},
		Holiday{On: NewDate(2026, time.June, 4), Name: "Factory holiday 4"},
		Holiday{On: NewDate(2026, time.June, 5), Name: "Factory holiday 5"},
	)

	in := Input{
		Chain: DefaultChain(), Settings: DefaultSettings(),
		ShipDate: NewDate(2026, time.June, 30),
		PODate:   NewDate(2026, time.February, 2),
		AsOf:     NewDate(2026, time.February, 10),
	}

	in.Calendar = plain
	before, err := Schedule(in)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	in.Calendar = shut
	after, err := Schedule(in)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if want := NewDate(2026, time.March, 25); before.LatestPODate != want {
		t.Errorf("with no holidays the order had to be placed by %v, want %v", before.LatestPODate, want)
	}
	// Five working days lost, crossing one weekly off, is six calendar days.
	if want := NewDate(2026, time.March, 19); after.LatestPODate != want {
		t.Errorf("with five holidays the order had to be placed by %v, want %v", after.LatestPODate, want)
	}
	if got, want := before.WorkingDays-after.WorkingDays, 5; got != want {
		t.Errorf("the plan lost %d working days to the holidays, want %d", got, want)
	}
	if got, want := after.ClosedDays-before.ClosedDays, 5; got != want {
		t.Errorf("closed days rose by %d, want %d", got, want)
	}

	// The sewing window itself has to start earlier, and the holidays inside it
	// are counted as closed days against it.
	sew := milestone(t, after, StageSewing)
	if want := NewDate(2026, time.May, 19); sew.PlannedStart != want {
		t.Errorf("sewing now starts %v, want %v", sew.PlannedStart, want)
	}
	if sew.ClosedDays < 5 {
		t.Errorf("the sewing window counts %d closed days, want at least the five holidays", sew.ClosedDays)
	}
}

func TestACalendarMilestoneIgnoresTheFactoryCalendar(t *testing.T) {
	// A vessel does not wait for a weekly off, so the transit and cut-off
	// milestones must not move when a holiday lands on them.
	plain := sundayOff()
	shut := sundayOff(
		Holiday{On: NewDate(2026, time.June, 25), Name: "On the transit"},
		Holiday{On: NewDate(2026, time.June, 26), Name: "On the transit"},
	)
	in := Input{Chain: DefaultChain(), Settings: DefaultSettings(),
		ShipDate: NewDate(2026, time.June, 30), PODate: NewDate(2026, time.February, 2), AsOf: NewDate(2026, time.February, 10)}

	in.Calendar = plain
	before, err := Schedule(in)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	in.Calendar = shut
	after, err := Schedule(in)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	for _, code := range []string{StageOnBoard, StageVesselCutOff, StageInlandTransit} {
		a, b := milestone(t, before, code), milestone(t, after, code)
		if a.PlannedStart != b.PlannedStart || a.PlannedFinish != b.PlannedFinish {
			t.Errorf("%s moved from %v..%v to %v..%v because of a factory holiday",
				code, a.PlannedStart, a.PlannedFinish, b.PlannedStart, b.PlannedFinish)
		}
	}
}

func TestOneMilestoneRunningLatePushesEverythingBehindIt(t *testing.T) {
	// Cutting was planned to finish Thursday 25 June and finished Friday 26
	// June instead. Sewing cannot start before Saturday 27 June, which pushes
	// it past the ship date, and the goods go on board a day late.
	p := tinyPlan(t, Input{
		PODate:   NewDate(2026, time.June, 22),
		AsOf:     NewDate(2026, time.June, 24),
		Settings: DefaultSettings(),
		Actuals:  map[string]Date{"CUT": NewDate(2026, time.June, 26)},
	})

	cut := milestone(t, p, "CUT")
	if !cut.Done || cut.Status != StatusDone {
		t.Errorf("cutting is done = %v with status %s, want it done", cut.Done, cut.Status)
	}
	if want := 1; cut.LateDays != want {
		t.Errorf("cutting finished %d days late, want %d", cut.LateDays, want)
	}

	sew := milestone(t, p, "SEW")
	if want := NewDate(2026, time.June, 27); sew.ProjectedStart != want {
		t.Errorf("sewing is now projected to start %v, want %v", sew.ProjectedStart, want)
	}
	if want := NewDate(2026, time.June, 30); sew.ProjectedFinish != want {
		t.Errorf("sewing is now projected to finish %v, want %v", sew.ProjectedFinish, want)
	}
	if sew.Status != StatusLate || sew.SlipDays != 1 {
		t.Errorf("sewing is %s and %d days past its planned finish, want late by 1", sew.Status, sew.SlipDays)
	}
	if sew.DrivenBy != "CUT" {
		t.Errorf("sewing is waiting on %q, want %q", sew.DrivenBy, "CUT")
	}

	ship := milestone(t, p, "SHIP")
	if want := NewDate(2026, time.July, 1); ship.ProjectedFinish != want {
		t.Errorf("the goods are now projected on board %v, want %v", ship.ProjectedFinish, want)
	}

	if want := NewDate(2026, time.July, 1); p.ProjectedShipDate != want {
		t.Errorf("projected ship date = %v, want %v", p.ProjectedShipDate, want)
	}
	if want := 1; p.SlipDays != want {
		t.Errorf("the order is %d days late, want %d", p.SlipDays, want)
	}
	if p.Status != StatusLate {
		t.Errorf("status = %s, want %s", p.Status, StatusLate)
	}
	// The order was always physically makeable: it is the slip that lost it.
	if !p.Feasible {
		t.Error("the order was called unmakeable, but it had the time at the start")
	}
	if p.ChaseCode != "SEW" {
		t.Errorf("the milestone to chase is %s, want SEW", p.ChaseCode)
	}
	if !strings.Contains(p.Verdict, "Running late") || !strings.Contains(p.Verdict, "1 day after") {
		t.Errorf("verdict = %q, want it to say the order is running late by one day", p.Verdict)
	}
}

func TestEveryPlannedDateIsALastPossibleDate(t *testing.T) {
	// There is no spare day hidden inside a backward plan: every date in it is
	// the latest the milestone can run. So a milestone that finishes after its
	// own date moves the ship date, unless days the factory is shut cover it.
	// This is the property the next test is the exception to, and it is worth
	// stating because a plan that looked like it had slack is how an order ends
	// up on an aeroplane.
	for _, late := range []int{1, 2, 5} {
		base := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22)})
		cut := milestone(t, base, "CUT")

		slipped := tinyPlan(t, Input{
			PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22),
			Actuals: map[string]Date{"CUT": cut.PlannedFinish.AddDays(late)},
		})
		if slipped.SlipDays == 0 {
			t.Errorf("cutting finished %d days past its own date and the ship date did not move", late)
		}
		if slipped.Status != StatusLate {
			t.Errorf("cutting finished %d days late and the order reads %s, want %s", late, slipped.Status, StatusLate)
		}
	}
}

func TestAShutdownCoversASlipWithoutTheOrderGoingLate(t *testing.T) {
	// Where the slack does live: the days the factory is shut. Sewing cannot
	// start until Friday 26 June whatever happens, because Wednesday and
	// Thursday are declared holidays and Sunday is the weekly off. That gives
	// cutting two days of cover beyond its own date — and the trims, which
	// arrive by lorry and do not keep the factory's calendar, do not get it.
	//
	// So cutting finishing on Thursday 25 June, two days past its date, costs
	// the order nothing. It is still not something to be left alone, so the
	// order reads at risk rather than on track. That difference is the whole
	// reason for working the dates out instead of colouring a spreadsheet.
	chain := Chain{Name: "Join", Stages: []Stage{
		{Code: "CUT", Name: "Cut", Owner: "Cutting room", Days: 2, Basis: WorkingDays},
		{Code: "TRIMS", Name: "Trims in-house", Owner: "Trims supplier", Days: 1, Basis: CalendarDays},
		{Code: "SEW", Name: "Sew", Owner: "Production", Days: 3, Basis: WorkingDays, After: []string{"CUT", "TRIMS"}},
		{Code: "SHIP", Name: "On board", Owner: "Carrier", Days: 1, Basis: CalendarDays, After: []string{"SEW"}},
	}}
	cal := sundayOff(
		Holiday{On: NewDate(2026, time.June, 24), Name: "Factory shut"},
		Holiday{On: NewDate(2026, time.June, 25), Name: "Factory shut"},
	)

	in := Input{
		Chain: chain, Calendar: cal, Settings: DefaultSettings(),
		ShipDate: NewDate(2026, time.June, 30),
		PODate:   NewDate(2026, time.June, 18),
		AsOf:     NewDate(2026, time.June, 23),
	}
	plan, err := Schedule(in)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	// The dates the shutdown produces, before anything is recorded.
	if want := NewDate(2026, time.June, 23); milestone(t, plan, "CUT").PlannedFinish != want {
		t.Fatalf("cutting's date = %v, want %v", milestone(t, plan, "CUT").PlannedFinish, want)
	}
	if want := NewDate(2026, time.June, 25); milestone(t, plan, "TRIMS").PlannedFinish != want {
		t.Fatalf("the trims date = %v, want %v", milestone(t, plan, "TRIMS").PlannedFinish, want)
	}
	if want := NewDate(2026, time.June, 26); milestone(t, plan, "SEW").PlannedStart != want {
		t.Fatalf("sewing starts %v, want %v", milestone(t, plan, "SEW").PlannedStart, want)
	}

	in.Actuals = map[string]Date{
		"CUT":   NewDate(2026, time.June, 25),
		"TRIMS": NewDate(2026, time.June, 25),
	}
	after, err := Schedule(in)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	cut := milestone(t, after, "CUT")
	if want := 2; cut.LateDays != want {
		t.Errorf("cutting finished %d days past its date, want %d", cut.LateDays, want)
	}
	sew := milestone(t, after, "SEW")
	if sew.SlipDays != 0 || sew.ProjectedStart != NewDate(2026, time.June, 26) {
		t.Errorf("sewing is projected to start %v and slip %d days, want 2026-06-26 and no slip",
			sew.ProjectedStart, sew.SlipDays)
	}
	if after.SlipDays != 0 || after.ProjectedShipDate != in.ShipDate {
		t.Errorf("the order is projected on board %v, %d days late; want the ship date and no slip",
			after.ProjectedShipDate, after.SlipDays)
	}
	if after.Status != StatusAtRisk {
		t.Errorf("status = %s, want %s: nothing was lost, but something has already gone past its date",
			after.Status, StatusAtRisk)
	}
	if want := 1; after.PastDueCount != want {
		t.Errorf("%d milestones have gone past their date, want %d", after.PastDueCount, want)
	}
	if !strings.Contains(after.Verdict, "gone past their own date") {
		t.Errorf("verdict = %q, want it to say why the order is at risk", after.Verdict)
	}

	// The milestone to chase is the next one due, not the one with the least
	// slack: sewing comes before going on board.
	if after.ChaseCode != "SEW" {
		t.Errorf("the milestone to chase is %s, want SEW", after.ChaseCode)
	}
}

func TestTheMilestoneToChaseIsTheNextOneDueWhenNothingHasSlipped(t *testing.T) {
	// Everything up to cutting is recorded on plan. The next thing the
	// merchandiser has to push is sewing, not the milestone furthest down the
	// chain that happens to carry the least slack.
	p := tinyPlan(t, Input{
		PODate: NewDate(2026, time.June, 1), AsOf: NewDate(2026, time.June, 10), Settings: DefaultSettings(),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.June, 10)},
	})
	if p.SlipDays != 0 {
		t.Fatalf("the order slips %d days; the fixture should be on time", p.SlipDays)
	}
	if p.ChaseCode != "SEW" {
		t.Errorf("the milestone to chase is %s, want SEW", p.ChaseCode)
	}
	if want := NewDate(2026, time.June, 29); p.ChaseDue != want {
		t.Errorf("chase due %v, want %v", p.ChaseDue, want)
	}
}

func TestPastDueCount(t *testing.T) {
	// Cutting's date is behind us with nothing recorded, and sewing is due
	// today, which is not yet past due.
	p := tinyPlan(t, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 29), Settings: DefaultSettings(),
	})
	if want := 1; p.PastDueCount != want {
		t.Errorf("%d milestones have gone past their date, want %d", p.PastDueCount, want)
	}

	// Nothing past due on an order that has not started yet.
	fresh := tinyPlan(t, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22), Settings: DefaultSettings(),
	})
	if fresh.PastDueCount != 0 {
		t.Errorf("%d milestones have gone past their date on a fresh order, want none", fresh.PastDueCount)
	}
}

func TestAMilestoneWhoseDateHasPassedIsFlaggedOverdueAndLate(t *testing.T) {
	// Nothing recorded, and the day is already past the planned finish of the
	// first two milestones.
	p := tinyPlan(t, Input{
		PODate:   NewDate(2026, time.June, 22),
		AsOf:     NewDate(2026, time.June, 29),
		Settings: DefaultSettings(),
	})

	cut := milestone(t, p, "CUT")
	if !cut.Overdue {
		t.Errorf("cutting was planned to finish %v and nothing is recorded, but it is not flagged overdue", cut.PlannedFinish)
	}
	if cut.Status != StatusLate {
		t.Errorf("cutting status = %s, want %s", cut.Status, StatusLate)
	}
	if want := 5; cut.SlipDays != want {
		t.Errorf("cutting is now %d days past its planned finish, want %d", cut.SlipDays, want)
	}
	// Work still to be done cannot be put in the past.
	if cut.ProjectedStart.Before(p.AsOf) {
		t.Errorf("cutting is projected to start %v, which is before today, %v", cut.ProjectedStart, p.AsOf)
	}

	if want := NewDate(2026, time.July, 4); p.ProjectedShipDate != want {
		t.Errorf("projected ship date = %v, want %v", p.ProjectedShipDate, want)
	}
	if want := 4; p.SlipDays != want {
		t.Errorf("the order is %d days late, want %d", p.SlipDays, want)
	}
	if p.Status != StatusLate {
		t.Errorf("status = %s, want %s", p.Status, StatusLate)
	}
	if p.ChaseCode != "CUT" {
		t.Errorf("the milestone to chase is %s, want CUT", p.ChaseCode)
	}
}

func TestADoneMilestoneIsNeverOverdue(t *testing.T) {
	p := tinyPlan(t, Input{
		PODate:  NewDate(2026, time.June, 22),
		AsOf:    NewDate(2026, time.June, 29),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.June, 24)},
	})
	cut := milestone(t, p, "CUT")
	if cut.Overdue {
		t.Error("a milestone completed before its planned date is flagged overdue")
	}
	if cut.LateDays != 0 {
		t.Errorf("cutting is %d days late, want none: it finished a day early", cut.LateDays)
	}
}

func TestACompletedOrderReadsAsShipped(t *testing.T) {
	cases := []struct {
		name        string
		shipActual  Date
		wantSlip    int
		wantVerdict string
	}{
		{name: "on time", shipActual: NewDate(2026, time.June, 30), wantSlip: 0, wantVerdict: "on time"},
		{name: "two days late", shipActual: NewDate(2026, time.July, 2), wantSlip: 2, wantVerdict: "2 days after the ship date"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := tinyPlan(t, Input{
				PODate: NewDate(2026, time.June, 22),
				AsOf:   NewDate(2026, time.July, 3),
				Actuals: map[string]Date{
					"CUT":  NewDate(2026, time.June, 25),
					"SEW":  NewDate(2026, time.June, 29),
					"SHIP": c.shipActual,
				},
			})
			if !p.Complete {
				t.Fatalf("done %d of %d milestones, want all of them", p.DoneCount, p.TotalCount)
			}
			if p.Status != StatusDone {
				t.Errorf("status = %s, want %s", p.Status, StatusDone)
			}
			if p.SlipDays != c.wantSlip {
				t.Errorf("shipped %d days late, want %d", p.SlipDays, c.wantSlip)
			}
			if !strings.Contains(p.Verdict, "Shipped") || !strings.Contains(p.Verdict, c.wantVerdict) {
				t.Errorf("verdict = %q, want it to mention %q", p.Verdict, c.wantVerdict)
			}
			if p.ChaseCode != "" {
				t.Errorf("a finished order still names %s to chase", p.ChaseCode)
			}
		})
	}
}

func TestTheAtRiskThresholdIsSettable(t *testing.T) {
	cases := []struct {
		name  string
		float int
		want  Status
	}{
		{name: "the shipped threshold calls three days of slack at risk", float: DefaultAtRiskFloatDays, want: StatusAtRisk},
		{name: "a tighter threshold calls the same order on track", float: 1, want: StatusOnTrack},
		{name: "a looser threshold calls it at risk", float: 10, want: StatusAtRisk},
		{name: "a threshold below one falls back to the shipped figure", float: 0, want: StatusAtRisk},
		{name: "a negative threshold falls back to the shipped figure", float: -5, want: StatusAtRisk},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// This order has exactly three days of slack.
			p := tinyPlan(t, Input{
				PODate:   NewDate(2026, time.June, 22),
				AsOf:     NewDate(2026, time.June, 22),
				Settings: Settings{AtRiskFloatDays: c.float},
			})
			if p.MinFloatDays != 3 {
				t.Fatalf("the order has %d days of slack, want 3", p.MinFloatDays)
			}
			if p.Status != c.want {
				t.Errorf("status = %s, want %s", p.Status, c.want)
			}
		})
	}
}

func TestSettingsNormalise(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{in: 0, want: DefaultAtRiskFloatDays},
		{in: -1, want: DefaultAtRiskFloatDays},
		{in: 1, want: 1},
		{in: 7, want: 7},
		{in: 400, want: 365},
	}
	for _, c := range cases {
		got := Settings{AtRiskFloatDays: c.in}.Normalise()
		if got.AtRiskFloatDays != c.want {
			t.Errorf("Settings{%d}.Normalise() = %d, want %d", c.in, got.AtRiskFloatDays, c.want)
		}
	}
}

func TestWithoutAPurchaseOrderDateThePlanStartsFromTheLatestItCouldHave(t *testing.T) {
	p := tinyPlan(t, Input{AsOf: NewDate(2026, time.June, 1), Settings: DefaultSettings()})

	if p.PODateKnown {
		t.Error("the plan claims to know a purchase-order date")
	}
	if p.PlanStart != p.LatestPODate {
		t.Errorf("the plan starts %v, want the latest possible order date %v", p.PlanStart, p.LatestPODate)
	}
	if p.MinFloatDays != 0 {
		t.Errorf("the order's slack = %d days, want none when there is nothing to measure it from", p.MinFloatDays)
	}
	// With no order date there is no measure of slack, so thin slack must not
	// be read into it.
	if p.Status != StatusOnTrack {
		t.Errorf("status = %s, want %s", p.Status, StatusOnTrack)
	}
	if !p.Feasible {
		t.Error("an order with no purchase-order date was called unmakeable")
	}

	joined := strings.Join(p.Warnings, " ")
	if !strings.Contains(joined, "No purchase-order date recorded") {
		t.Errorf("warnings = %v, want one about the missing order date", p.Warnings)
	}
}

func TestWarnings(t *testing.T) {
	t.Run("an order placed too late to make the ship date", func(t *testing.T) {
		p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 29), AsOf: NewDate(2026, time.June, 29)})
		joined := strings.Join(p.Warnings, " ")
		if !strings.Contains(joined, "last day it could have been placed") {
			t.Errorf("warnings = %v, want one about the order being placed too late", p.Warnings)
		}
	})

	t.Run("an empty holiday list", func(t *testing.T) {
		p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22)})
		joined := strings.Join(p.Warnings, " ")
		if !strings.Contains(joined, "No holidays") {
			t.Errorf("warnings = %v, want one about the empty holiday list", p.Warnings)
		}
	})

	t.Run("no warning once holidays are entered", func(t *testing.T) {
		p := tinyPlan(t, Input{
			Calendar: sundayOff(Holiday{On: NewDate(2026, time.June, 10), Name: "Holiday"}),
			PODate:   NewDate(2026, time.June, 1), AsOf: NewDate(2026, time.June, 1),
		})
		joined := strings.Join(p.Warnings, " ")
		if strings.Contains(joined, "No holidays") {
			t.Errorf("warnings = %v, want nothing about the holiday list", p.Warnings)
		}
	})

	t.Run("progress recorded against a milestone that is not in the chain", func(t *testing.T) {
		p := tinyPlan(t, Input{
			PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22),
			Actuals: map[string]Date{"EMBROIDERY": NewDate(2026, time.June, 23)},
		})
		joined := strings.Join(p.Warnings, " ")
		if !strings.Contains(joined, "EMBROIDERY") {
			t.Errorf("warnings = %v, want one naming the unknown milestone", p.Warnings)
		}
		if p.DoneCount != 0 {
			t.Errorf("done %d milestones, want none: the entry belongs to no milestone here", p.DoneCount)
		}
	})

	t.Run("a milestone completed before the one in front of it", func(t *testing.T) {
		p := tinyPlan(t, Input{
			PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22),
			Actuals: map[string]Date{"SEW": NewDate(2026, time.June, 26)},
		})
		joined := strings.Join(p.Warnings, " ")
		if !strings.Contains(joined, "SEW is marked complete but CUT") {
			t.Errorf("warnings = %v, want one saying sewing is done while cutting is not", p.Warnings)
		}
	})
}

func TestACompletionDateIsTakenAtFaceValue(t *testing.T) {
	// Sewing recorded as complete on a Sunday. The factory may well have run a
	// shift; the tool records what happened rather than correcting the user.
	sunday := NewDate(2026, time.June, 28)
	p := tinyPlan(t, Input{
		PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 29),
		Actuals: map[string]Date{"CUT": NewDate(2026, time.June, 25), "SEW": sunday},
	})
	sew := milestone(t, p, "SEW")
	if sew.Actual != sunday || sew.ProjectedFinish != sunday {
		t.Errorf("sewing finished %v and is projected to finish %v, want %v for both", sew.Actual, sew.ProjectedFinish, sunday)
	}
	if sew.LateDays != 0 {
		t.Errorf("sewing is %d days late, want none: it finished a day before plan", sew.LateDays)
	}
	ship := milestone(t, p, "SHIP")
	if want := NewDate(2026, time.June, 29); ship.ProjectedFinish != want {
		t.Errorf("the goods are projected on board %v, want %v", ship.ProjectedFinish, want)
	}
}

func TestAZeroAsOfDateMeansToday(t *testing.T) {
	p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22)})
	if p.AsOf != Today() {
		t.Errorf("as-of date = %v, want today, %v", p.AsOf, Today())
	}
}

func TestDaysToShipCountsFromToday(t *testing.T) {
	p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 20)})
	if want := 10; p.DaysToShip != want {
		t.Errorf("days to ship = %d, want %d", p.DaysToShip, want)
	}
}

func TestMilestoneCountsAndChainName(t *testing.T) {
	p := tinyPlan(t, Input{PODate: NewDate(2026, time.June, 22), AsOf: NewDate(2026, time.June, 22)})
	if p.TotalCount != 3 || len(p.Milestones) != 3 {
		t.Errorf("the plan has %d milestones and counts %d, want 3", len(p.Milestones), p.TotalCount)
	}
	if p.ChainName != "Tiny" {
		t.Errorf("chain name = %q, want %q", p.ChainName, "Tiny")
	}
	if _, ok := p.Find("  sew "); !ok {
		t.Error("Find did not match a code written loosely")
	}
}

func TestStatusLabelAndRank(t *testing.T) {
	cases := []struct {
		status    Status
		wantLabel string
		wantRank  int
	}{
		{StatusLate, "Late", 0},
		{StatusAtRisk, "At risk", 1},
		{StatusOnTrack, "On track", 2},
		{StatusDone, "Done", 3},
	}
	for _, c := range cases {
		if got := c.status.Label(); got != c.wantLabel {
			t.Errorf("%s.Label() = %q, want %q", c.status, got, c.wantLabel)
		}
		if got := c.status.Rank(); got != c.wantRank {
			t.Errorf("%s.Rank() = %d, want %d", c.status, got, c.wantRank)
		}
	}
}
