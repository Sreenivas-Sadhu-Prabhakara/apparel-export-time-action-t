package timeaction

import (
	"errors"
	"fmt"
	"sort"
)

// Status is how a milestone, or a whole order, is reading.
type Status string

const (
	// StatusOnTrack means the plan still holds.
	StatusOnTrack Status = "on-track"
	// StatusAtRisk means the plan holds but there is little or no slack left.
	StatusAtRisk Status = "at-risk"
	// StatusLate means the plan no longer holds: this milestone, or this
	// order, cannot be met on the dates it was planned to.
	StatusLate Status = "late"
	// StatusDone means the work was recorded as finished.
	StatusDone Status = "done"
)

// Label names a status for a screen or a report.
func (s Status) Label() string {
	switch s {
	case StatusDone:
		return "Done"
	case StatusLate:
		return "Late"
	case StatusAtRisk:
		return "At risk"
	default:
		return "On track"
	}
}

// Rank orders the statuses worst first, for sorting a board.
func (s Status) Rank() int {
	switch s {
	case StatusLate:
		return 0
	case StatusAtRisk:
		return 1
	case StatusOnTrack:
		return 2
	default:
		return 3
	}
}

// DefaultAtRiskFloatDays is the slack, in calendar days, at or below which a
// milestone is called at risk rather than on track.
//
// It is a judgement about how much warning a planner wants, not a figure out of
// any rule book, and it is editable.
const DefaultAtRiskFloatDays = 3

// Settings are the thresholds a plan is read by.
type Settings struct {
	AtRiskFloatDays int `json:"atRiskFloatDays"`
}

// DefaultSettings is what this build starts with.
func DefaultSettings() Settings { return Settings{AtRiskFloatDays: DefaultAtRiskFloatDays} }

// Normalise keeps the thresholds inside the range they mean anything in. A
// milestone with no slack at all is always at risk, so the floor is one day.
func (s Settings) Normalise() Settings {
	if s.AtRiskFloatDays < 1 {
		s.AtRiskFloatDays = DefaultAtRiskFloatDays
	}
	if s.AtRiskFloatDays > 365 {
		s.AtRiskFloatDays = 365
	}
	return s
}

// Input is everything one plan is worked out from.
type Input struct {
	Chain    Chain
	Calendar Calendar
	Settings Settings

	// ShipDate is the date the goods must be on board. The whole plan is
	// scheduled backwards from it.
	ShipDate Date

	// PODate is the date the purchase order was placed. It is optional: without
	// it the slack is measured against the latest date the order could have
	// been placed, and no feasibility verdict is given.
	PODate Date

	// Actuals holds the dates milestones actually finished, by milestone code.
	Actuals map[string]Date

	// AsOf is the day the plan is being read on. Zero means today.
	AsOf Date
}

// Milestone is one line of a worked-out plan.
type Milestone struct {
	Code  string   `json:"code"`
	Name  string   `json:"name"`
	Owner string   `json:"owner"`
	Note  string   `json:"note"`
	Basis Basis    `json:"basis"`
	Days  int      `json:"days"`
	After []string `json:"after"`

	// PlannedStart and PlannedFinish come from the backward pass: the latest
	// this milestone can run and still make the ship date. These are the dates
	// the plan publishes.
	PlannedStart  Date `json:"plannedStart"`
	PlannedFinish Date `json:"plannedFinish"`

	// EarliestStart and EarliestFinish come from the forward pass off the
	// purchase-order date, taking no account of what has actually happened.
	EarliestStart  Date `json:"earliestStart"`
	EarliestFinish Date `json:"earliestFinish"`

	// ProjectedStart and ProjectedFinish are where this milestone now lands,
	// given what has actually been completed and that nothing can be done
	// before today.
	ProjectedStart  Date `json:"projectedStart"`
	ProjectedFinish Date `json:"projectedFinish"`

	Actual Date `json:"actual"`
	Done   bool `json:"done"`

	// FloatDays is the slack in this milestone's own basis — working days for a
	// factory stage, calendar days for transit — because that is the unit the
	// slack can actually be spent in.
	FloatDays int `json:"floatDays"`
	// FloatCalendarDays is the same slack in plain calendar days, which is the
	// figure the critical path is found by.
	FloatCalendarDays int `json:"floatCalendarDays"`

	Critical   bool   `json:"critical"`
	Overdue    bool   `json:"overdue"`
	SlipDays   int    `json:"slipDays"`
	LateDays   int    `json:"lateDays"`
	ClosedDays int    `json:"closedDays"`
	DrivenBy   string `json:"drivenBy"`
	Status     Status `json:"status"`
}

// Plan is a whole order's worked-out time and action.
type Plan struct {
	ChainName string `json:"chainName"`
	ShipDate  Date   `json:"shipDate"`
	PODate    Date   `json:"poDate"`
	PlanStart Date   `json:"planStart"`
	AsOf      Date   `json:"asOf"`

	Milestones   []Milestone `json:"milestones"`
	CriticalPath []string    `json:"criticalPath"`
	MinFloatDays int         `json:"minFloatDays"`

	// LatestPODate is the last day the purchase order could have been placed
	// for this ship date and this chain.
	LatestPODate Date `json:"latestPoDate"`
	PODateKnown  bool `json:"poDateKnown"`

	// EarliestShipDate is the soonest the goods could go on board, counted
	// forward from the purchase-order date. ShortfallDays is how far that falls
	// past the ship date, which is the number of days the order is short by.
	EarliestShipDate Date `json:"earliestShipDate"`
	ShortfallDays    int  `json:"shortfallDays"`
	Feasible         bool `json:"feasible"`

	// ProjectedShipDate is where the order now lands on the progress recorded
	// against it. SlipDays is how far that falls past the ship date.
	ProjectedShipDate Date `json:"projectedShipDate"`
	SlipDays          int  `json:"slipDays"`
	DaysToShip        int  `json:"daysToShip"`

	Status     Status `json:"status"`
	Complete   bool   `json:"complete"`
	DoneCount  int    `json:"doneCount"`
	TotalCount int    `json:"totalCount"`

	// PastDueCount is how many milestones have already gone past their own
	// date: finished after it, or not finished with it behind us.
	PastDueCount int `json:"pastDueCount"`

	// ChaseCode names the one milestone that most threatens the ship date.
	ChaseCode  string `json:"chaseCode"`
	ChaseName  string `json:"chaseName"`
	ChaseOwner string `json:"chaseOwner"`
	ChaseDue   Date   `json:"chaseDue"`

	Verdict     string   `json:"verdict"`
	Warnings    []string `json:"warnings"`
	WorkingDays int      `json:"workingDays"`
	ClosedDays  int      `json:"closedDays"`
}

// Find returns a milestone of the plan by code.
func (p Plan) Find(code string) (Milestone, bool) {
	code = NormaliseCode(code)
	for _, m := range p.Milestones {
		if m.Code == code {
			return m, true
		}
	}
	return Milestone{}, false
}

// Schedule works out one order's plan.
//
// It makes three passes over the milestone chain:
//
// The backward pass runs from the ship date and gives every milestone the
// latest date it can finish on. Those are the planned dates — the ones on the
// sheet the merchandiser sends out.
//
// The forward pass runs from the purchase-order date and gives every milestone
// the earliest date it could finish on. The difference between the two is that
// milestone's float, the milestones with the least float are the critical path,
// and if the forward pass lands past the ship date the order cannot be made on
// time at all.
//
// The projection pass runs forward again, but starting from the dates
// milestones actually finished on and refusing to put any remaining work in the
// past. That is what moves the inspection and the booking when finishing runs
// late.
func Schedule(in Input) (Plan, error) {
	ch := in.Chain.Normalise()
	if err := ch.Validate(); err != nil {
		return Plan{}, err
	}
	cal := in.Calendar.Normalise()
	if err := cal.Validate(); err != nil {
		return Plan{}, err
	}
	if in.ShipDate.IsZero() {
		return Plan{}, errors.New("the plan is scheduled backwards from the ship date, so the ship date is the one figure it cannot do without")
	}
	set := in.Settings.Normalise()

	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = Today()
	}

	order, err := ch.TopologicalOrder()
	if err != nil {
		return Plan{}, err
	}
	successors := ch.Successors()
	terminals := ch.Terminals()
	actuals, unknown := splitActuals(ch, in.Actuals)

	// Backward pass: the latest every milestone can run.
	lateStart := map[string]Date{}
	lateFinish := map[string]Date{}
	for i := len(order) - 1; i >= 0; i-- {
		s := order[i]
		var finish Date
		if waiting := successors[s.Code]; len(waiting) == 0 {
			finish = cal.SnapBack(in.ShipDate, s.Basis)
		} else {
			for _, t := range waiting {
				finish = earliest(finish, cal.PrevDay(lateStart[t], s.Basis))
			}
		}
		lateFinish[s.Code] = finish
		lateStart[s.Code] = cal.SpanStart(finish, s.Days, s.Basis)
	}

	// The latest the order could have been placed is the latest start of the
	// milestones that wait on nothing.
	var latestPO Date
	for _, code := range ch.Roots() {
		latestPO = earliest(latestPO, lateStart[code])
	}

	planStart := in.PODate
	if planStart.IsZero() {
		planStart = latestPO
	}

	// Forward pass: the earliest every milestone could run, from the order date.
	earlyStart := map[string]Date{}
	earlyFinish := map[string]Date{}
	for _, s := range order {
		var start Date
		if len(s.After) == 0 {
			start = cal.Snap(planStart, s.Basis)
		} else {
			for _, p := range s.After {
				start = latest(start, cal.NextDay(earlyFinish[p], s.Basis))
			}
		}
		earlyStart[s.Code] = start
		earlyFinish[s.Code] = cal.SpanEnd(start, s.Days, s.Basis)
	}

	var earliestShip Date
	for _, code := range terminals {
		earliestShip = latest(earliestShip, earlyFinish[code])
	}
	shortfall := DaysBetween(in.ShipDate, earliestShip)
	if shortfall < 0 {
		shortfall = 0
	}

	// Float per milestone: how far its earliest run sits from its latest one.
	floatDays := map[string]int{}
	for _, s := range order {
		floatDays[s.Code] = DaysBetween(earlyStart[s.Code], lateStart[s.Code])
	}

	// The order's own float is the gap between the soonest it could be on
	// board and the day it has to be. Negative means it cannot be made in time.
	projectFloat := DaysBetween(earliestShip, in.ShipDate)

	// The critical path is the longest chain of work through the network, and
	// it is traced rather than picked out by float. Float alone will not do it
	// here: a chain that mixes stages counted in working days with stages
	// counted in calendar days picks up a day here and there as a span snaps
	// over a weekly off, so the milestones with the least float are not
	// necessarily a connected chain. Tracing the predecessor that is actually
	// holding each milestone up always gives one.
	path := criticalPath(ch, cal, earlyStart, earlyFinish, terminals)
	critical := map[string]bool{}
	for _, code := range path {
		critical[code] = true
	}

	// Projection pass: where the order lands on the progress recorded.
	projStart := map[string]Date{}
	projFinish := map[string]Date{}
	drivenBy := map[string]string{}
	for _, s := range order {
		if done, ok := actuals[s.Code]; ok {
			projFinish[s.Code] = done
			projStart[s.Code] = cal.SpanStart(done, s.Days, s.Basis)
			continue
		}

		var fromPreds Date
		binding := ""
		if len(s.After) == 0 {
			fromPreds = cal.Snap(planStart, s.Basis)
		} else {
			for _, p := range s.After {
				candidate := cal.NextDay(projFinish[p], s.Basis)
				if fromPreds.IsZero() || candidate.After(fromPreds) {
					fromPreds = candidate
					binding = p
				}
			}
		}

		// Work still to do cannot be done in the past.
		floor := cal.Snap(asOf, s.Basis)
		start := latest(fromPreds, floor)
		if !fromPreds.IsZero() && !fromPreds.Before(start) {
			drivenBy[s.Code] = binding
		}

		projStart[s.Code] = start
		projFinish[s.Code] = cal.SpanEnd(start, s.Days, s.Basis)
	}

	var projectedShip Date
	for _, code := range terminals {
		projectedShip = latest(projectedShip, projFinish[code])
	}
	slip := DaysBetween(in.ShipDate, projectedShip)
	if slip < 0 {
		slip = 0
	}

	plan := Plan{
		ChainName:         ch.Name,
		ShipDate:          in.ShipDate,
		PODate:            in.PODate,
		PlanStart:         planStart,
		AsOf:              asOf,
		MinFloatDays:      projectFloat,
		CriticalPath:      path,
		LatestPODate:      latestPO,
		PODateKnown:       !in.PODate.IsZero(),
		EarliestShipDate:  earliestShip,
		ShortfallDays:     shortfall,
		Feasible:          shortfall == 0,
		ProjectedShipDate: projectedShip,
		SlipDays:          slip,
		DaysToShip:        DaysBetween(asOf, in.ShipDate),
		TotalCount:        len(ch.Stages),
		WorkingDays:       cal.WorkingDaysBetween(planStart, in.ShipDate),
	}
	plan.ClosedDays, _ = cal.ClosedDaysBetween(planStart, in.ShipDate)

	// The milestones, in the order the chain holds them rather than in
	// dependency order, so an edited chain still reads the way it was arranged.
	for _, s := range ch.Stages {
		m := Milestone{
			Code:              s.Code,
			Name:              s.Name,
			Owner:             s.Owner,
			Note:              s.Note,
			Basis:             s.Basis,
			Days:              s.Days,
			After:             s.After,
			PlannedStart:      lateStart[s.Code],
			PlannedFinish:     lateFinish[s.Code],
			EarliestStart:     earlyStart[s.Code],
			EarliestFinish:    earlyFinish[s.Code],
			ProjectedStart:    projStart[s.Code],
			ProjectedFinish:   projFinish[s.Code],
			FloatCalendarDays: floatDays[s.Code],
			FloatDays:         cal.Drift(earlyStart[s.Code], lateStart[s.Code], s.Basis),
			Critical:          critical[s.Code],
			DrivenBy:          drivenBy[s.Code],
		}
		m.ClosedDays, _ = cal.ClosedDaysBetween(m.PlannedStart, m.PlannedFinish)

		if done, ok := actuals[s.Code]; ok {
			m.Done = true
			m.Actual = done
			m.Status = StatusDone
			if late := DaysBetween(m.PlannedFinish, done); late > 0 {
				m.LateDays = late
			}
			plan.DoneCount++
		} else {
			if slipped := DaysBetween(m.PlannedFinish, m.ProjectedFinish); slipped > 0 {
				m.SlipDays = slipped
			}
			m.Overdue = m.PlannedFinish.Before(asOf)
			switch {
			case m.SlipDays > 0:
				m.Status = StatusLate
			case m.FloatCalendarDays <= set.AtRiskFloatDays:
				m.Status = StatusAtRisk
			default:
				m.Status = StatusOnTrack
			}
		}
		if (m.Done && m.LateDays > 0) || (!m.Done && m.Overdue) {
			plan.PastDueCount++
		}
		plan.Milestones = append(plan.Milestones, m)
	}

	plan.Complete = plan.DoneCount == plan.TotalCount && plan.TotalCount > 0
	plan.Status = orderStatus(plan, set)
	plan.setChase()
	plan.Warnings = warnings(ch, cal, plan, actuals, unknown)
	plan.Verdict = plan.verdict()
	return plan, nil
}

// orderStatus rolls the milestones up into one word for the board.
//
// An order is late when the goods are projected to go on board after the ship
// date, or when they could never have made it. A milestone running late that
// the plan can still absorb makes the order at risk, not late: that difference
// is the whole point of working the slack out.
func orderStatus(p Plan, set Settings) Status {
	if p.Complete {
		return StatusDone
	}
	if p.SlipDays > 0 || !p.Feasible {
		return StatusLate
	}
	// Something has already gone past its date. The plan still reaches the ship
	// date, but it is no longer a plan anybody should leave alone.
	for _, m := range p.Milestones {
		if m.Status == StatusLate || (!m.Done && m.Overdue) || (m.Done && m.LateDays > 0) {
			return StatusAtRisk
		}
	}
	// Without a purchase-order date there is no measure of the order's own
	// slack, so it is not read as thin slack.
	if p.PODateKnown && p.MinFloatDays <= set.AtRiskFloatDays {
		return StatusAtRisk
	}
	return StatusOnTrack
}

// setChase picks the one milestone to go after: whatever is furthest behind,
// then whatever is on the critical path, then whatever is due soonest, then
// whatever has the least slack. A board that names twelve problems names none.
func (p *Plan) setChase() {
	var best *Milestone
	for i := range p.Milestones {
		m := &p.Milestones[i]
		if m.Done {
			continue
		}
		if best == nil {
			best = m
			continue
		}
		if m.SlipDays != best.SlipDays {
			if m.SlipDays > best.SlipDays {
				best = m
			}
			continue
		}
		if m.Critical != best.Critical {
			if m.Critical {
				best = m
			}
			continue
		}
		if m.PlannedFinish != best.PlannedFinish {
			if m.PlannedFinish.Before(best.PlannedFinish) {
				best = m
			}
			continue
		}
		if m.FloatCalendarDays < best.FloatCalendarDays {
			best = m
		}
	}
	if best == nil {
		return
	}
	p.ChaseCode = best.Code
	p.ChaseName = best.Name
	p.ChaseOwner = best.Owner
	p.ChaseDue = best.PlannedFinish
}

// verdict is the plan in one sentence, for the top of the screen and for the
// board export.
func (p Plan) verdict() string {
	switch {
	case p.Complete:
		if p.SlipDays > 0 {
			return fmt.Sprintf("Shipped %s, %s after the ship date.", p.ProjectedShipDate.Long(), days(p.SlipDays))
		}
		return fmt.Sprintf("Shipped %s, on time.", p.ProjectedShipDate.Long())
	case !p.Feasible:
		return fmt.Sprintf("Cannot be made on time: from a purchase order placed %s the earliest this can go on board is %s, %s after the ship date.",
			p.PODate.Long(), p.EarliestShipDate.Long(), days(p.ShortfallDays))
	case p.SlipDays > 0:
		return fmt.Sprintf("Running late: on the progress recorded the goods go on board %s, %s after the ship date. %s is the milestone to chase.",
			p.ProjectedShipDate.Long(), days(p.SlipDays), p.ChaseName)
	case !p.PODateKnown:
		return fmt.Sprintf("Planned backwards from %s. The purchase order had to be placed by %s. %s is the milestone to chase, due %s.",
			p.ShipDate.Long(), p.LatestPODate.Long(), p.ChaseName, p.ChaseDue.Long())
	case p.Status == StatusAtRisk && p.PastDueCount > 0:
		return fmt.Sprintf("At risk: %d of %d milestones have gone past their own date, and the ship date still holds on the days the factory is shut. %s is the milestone to chase, due %s.",
			p.PastDueCount, p.TotalCount, p.ChaseName, p.ChaseDue.Long())
	case p.Status == StatusAtRisk:
		return fmt.Sprintf("At risk: only %s of slack between the earliest this can be made and the ship date. %s is the milestone to chase, due %s.",
			days(p.MinFloatDays), p.ChaseName, p.ChaseDue.Long())
	default:
		return fmt.Sprintf("On track: %s of slack against the ship date. %s is the milestone to chase, due %s.",
			days(p.MinFloatDays), p.ChaseName, p.ChaseDue.Long())
	}
}

// days writes a day count the way a sentence needs it.
func days(n int) string {
	if n == 1 || n == -1 {
		return fmt.Sprintf("%d day", n)
	}
	return fmt.Sprintf("%d days", n)
}

// warnings states what the plan is standing on that the user may not have
// meant, rather than quietly assuming it.
func warnings(ch Chain, cal Calendar, p Plan, actuals map[string]Date, unknown []string) []string {
	var out []string

	if !p.PODateKnown {
		out = append(out, fmt.Sprintf("No purchase-order date recorded, so the slack is measured against %s, the last day the order could have been placed for this ship date.", p.LatestPODate))
	} else if p.PODate.After(p.LatestPODate) {
		out = append(out, fmt.Sprintf("The purchase order was placed %s, which is %s after %s, the last day it could have been placed for this ship date.",
			p.PODate, days(DaysBetween(p.LatestPODate, p.PODate)), p.LatestPODate))
	}

	if len(cal.Holidays) == 0 {
		out = append(out, "No holidays are on the calendar, so every day except the weekly off is counted as a working day. Holidays are where a plan loses days quietly — enter yours.")
	}

	for _, code := range unknown {
		out = append(out, fmt.Sprintf("A completion date is recorded against %s, which is not a milestone in this chain, so it is ignored.", code))
	}

	// A milestone marked complete while something it waits on is not tells the
	// planner their own records disagree, which the dates alone would hide.
	for _, s := range ch.Stages {
		if _, done := actuals[s.Code]; !done {
			continue
		}
		for _, pred := range s.After {
			if _, predDone := actuals[pred]; !predDone {
				out = append(out, fmt.Sprintf("%s is marked complete but %s, which comes before it, is not.", s.Code, pred))
			}
		}
	}
	return out
}

// splitActuals keeps the completion dates that belong to the chain and reports
// the rest, so a chain edited after progress was recorded says so rather than
// losing the entry in silence.
func splitActuals(ch Chain, in map[string]Date) (map[string]Date, []string) {
	kept := map[string]Date{}
	var unknown []string
	for code, on := range in {
		code = NormaliseCode(code)
		if on.IsZero() || code == "" {
			continue
		}
		if _, ok := ch.Find(code); !ok {
			unknown = append(unknown, code)
			continue
		}
		kept[code] = on
	}
	sort.Strings(unknown)
	return kept, unknown
}

// criticalPath traces the longest chain of work through the network: it starts
// at the milestone that sets the earliest possible ship date and walks back,
// at each step to the predecessor whose finish is the reason that milestone
// cannot start any sooner. The result is always a connected chain from a
// milestone that waits on nothing to the one the ship date hangs on, and it is
// the chain on which a day lost is a day lost off the ship date.
func criticalPath(ch Chain, cal Calendar, earlyStart, earlyFinish map[string]Date, terminals []string) []string {
	end := ""
	for _, code := range terminals {
		if end == "" || earlyFinish[code].After(earlyFinish[end]) {
			end = code
		}
	}

	var reversed []string
	walked := map[string]bool{}
	for code := end; code != ""; {
		if walked[code] {
			// Unreachable for a validated chain, which has no circles. Here so
			// that a bad chain cannot spin.
			break
		}
		walked[code] = true
		reversed = append(reversed, code)

		s, ok := ch.Find(code)
		if !ok {
			break
		}
		binding := ""
		for _, p := range s.After {
			if cal.NextDay(earlyFinish[p], s.Basis) != earlyStart[code] {
				continue
			}
			if binding == "" || earlyFinish[p].After(earlyFinish[binding]) {
				binding = p
			}
		}
		code = binding
	}

	out := make([]string, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		out = append(out, reversed[i])
	}
	return out
}
