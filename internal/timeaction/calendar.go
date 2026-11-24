package timeaction

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Basis says whether a lead time is counted in factory working days or in plain
// calendar days. Cutting and sewing stop for the weekly off and for a public
// holiday; a container on a trailer and a vessel at sea do not.
type Basis string

const (
	// WorkingDays counts only days the factory is open.
	WorkingDays Basis = "working"
	// CalendarDays counts every day, holidays included.
	CalendarDays Basis = "calendar"
)

// Normalise fills in the basis a hand-edited file left blank and accepts either
// spelling in any case. A value that is neither is left as it was found, so
// that Chain.Validate can refuse it instead of this quietly deciding that
// "fortnights" meant working days.
func (b Basis) Normalise() Basis {
	switch tidy := Basis(strings.ToLower(strings.TrimSpace(string(b)))); tidy {
	case CalendarDays:
		return CalendarDays
	case WorkingDays, "":
		return WorkingDays
	default:
		return tidy
	}
}

// Label names the basis for a screen or a report.
func (b Basis) Label() string {
	if b.Normalise() == CalendarDays {
		return "calendar days"
	}
	return "working days"
}

// Holiday is one day the factory is closed, named so the planner can see why a
// week lost a day.
type Holiday struct {
	On   Date   `json:"on"`
	Name string `json:"name"`
}

// Calendar is the factory's working week: the days it is closed every week, and
// the dated holidays on top of those.
//
// There is no shipped holiday list. Public holidays differ by country, by state
// and by district, factories declare their own, and a wrong list is worse than
// an empty one because it moves dates without being noticed. The user enters
// their own.
type Calendar struct {
	// WeeklyOff holds the weekdays the factory is closed, as Go weekday
	// numbers: Sunday is 0 and Saturday is 6.
	WeeklyOff []time.Weekday `json:"weeklyOff"`
	Holidays  []Holiday      `json:"holidays"`
}

// DefaultWeeklyOff is the one day a week most garment units are closed.
// Indicative only: a unit running a five-day week, or one with a rotating off,
// takes a different list, and this is editable.
func DefaultWeeklyOff() []time.Weekday { return []time.Weekday{time.Sunday} }

// DefaultCalendar is the working week this build starts with: one weekly off
// and no holidays, so that nothing is assumed about the user's own calendar.
func DefaultCalendar() Calendar {
	return Calendar{WeeklyOff: DefaultWeeklyOff()}.Normalise()
}

// Normalise sorts and de-duplicates the calendar so that two files holding the
// same working week compare equal and the arithmetic is deterministic.
func (c Calendar) Normalise() Calendar {
	seenDay := map[time.Weekday]bool{}
	var offs []time.Weekday
	for _, w := range c.WeeklyOff {
		if w < time.Sunday || w > time.Saturday || seenDay[w] {
			continue
		}
		seenDay[w] = true
		offs = append(offs, w)
	}
	sort.Slice(offs, func(i, j int) bool { return offs[i] < offs[j] })

	seenDate := map[Date]bool{}
	var days []Holiday
	for _, h := range c.Holidays {
		h.Name = strings.TrimSpace(h.Name)
		if h.On.IsZero() || seenDate[h.On] {
			continue
		}
		seenDate[h.On] = true
		if h.Name == "" {
			h.Name = "Holiday"
		}
		days = append(days, h)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].On.Before(days[j].On) })

	return Calendar{WeeklyOff: offs, Holidays: days}
}

// Validate refuses a calendar that leaves the factory no working day at all.
// Every other method relies on this: with at least one open weekday, a run of
// closed days is bounded and the arithmetic always terminates.
func (c Calendar) Validate() error {
	if len(c.Normalise().WeeklyOff) >= 7 {
		return errors.New("every day of the week is marked off, so no work could ever be done — leave at least one working day")
	}
	for _, h := range c.Holidays {
		if h.On.IsZero() {
			return errors.New("a holiday needs a date")
		}
	}
	return nil
}

// WorkingWeekDays is the number of days a week the factory is open.
func (c Calendar) WorkingWeekDays() int { return 7 - len(c.Normalise().WeeklyOff) }

// IsWeeklyOff reports whether a date falls on the weekly off.
func (c Calendar) IsWeeklyOff(d Date) bool {
	w := d.Weekday()
	for _, off := range c.WeeklyOff {
		if off == w {
			return true
		}
	}
	return false
}

// HolidayOn returns the holiday falling on a date, if there is one.
func (c Calendar) HolidayOn(d Date) (Holiday, bool) {
	for _, h := range c.Holidays {
		if h.On == d {
			return h, true
		}
	}
	return Holiday{}, false
}

// IsWorkingDay reports whether the factory is open on a date. A holiday that
// falls on the weekly off costs one day, not two.
func (c Calendar) IsWorkingDay(d Date) bool {
	if d.IsZero() {
		return false
	}
	if c.IsWeeklyOff(d) {
		return false
	}
	_, holiday := c.HolidayOn(d)
	return !holiday
}

// scanLimit bounds the search for the next open day.
//
// With w weekdays off out of seven, a run of n consecutive closed days needs at
// least n - ceil(n*w/7) dated holidays, so n cannot exceed 7*(holidays+1) once
// at least one weekday is open. Validate guarantees that, so the limit is never
// reached in practice; it is here so that an unvalidated calendar returns a
// wrong answer rather than hanging.
func (c Calendar) scanLimit() int { return 7*(len(c.Holidays)+1) + 7 }

// OnOrAfterWorking returns the first working day on or after a date.
func (c Calendar) OnOrAfterWorking(d Date) Date {
	if d.IsZero() {
		return d
	}
	limit := c.scanLimit()
	for i := 0; i <= limit; i++ {
		if c.IsWorkingDay(d) {
			return d
		}
		d = d.AddDays(1)
	}
	return d
}

// OnOrBeforeWorking returns the last working day on or before a date.
func (c Calendar) OnOrBeforeWorking(d Date) Date {
	if d.IsZero() {
		return d
	}
	limit := c.scanLimit()
	for i := 0; i <= limit; i++ {
		if c.IsWorkingDay(d) {
			return d
		}
		d = d.AddDays(-1)
	}
	return d
}

// ShiftWorking moves a date by a number of working days: forward for a positive
// count, backward for a negative one, and not at all for zero. Every step lands
// on a working day, so the result is a working day whenever the count is not
// zero.
func (c Calendar) ShiftWorking(d Date, n int) Date {
	if d.IsZero() || n == 0 {
		return d
	}
	step := 1
	if n < 0 {
		step = -1
		n = -n
	}
	for ; n > 0; n-- {
		d = d.AddDays(step)
		limit := c.scanLimit()
		for i := 0; !c.IsWorkingDay(d) && i <= limit; i++ {
			d = d.AddDays(step)
		}
	}
	return d
}

// WorkingDaysBetween counts the working days from one date to another, counting
// both ends. It is zero when the second date falls before the first, so an
// inverted range reads as "no days" rather than as a negative count.
func (c Calendar) WorkingDaysBetween(from, to Date) int {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return 0
	}
	n := 0
	for d := from; !d.After(to); d = d.AddDays(1) {
		if c.IsWorkingDay(d) {
			n++
		}
	}
	return n
}

// WorkingDaysFromTo returns the number of working days of movement from one
// date to another: zero for the same day, positive forward, negative backward.
// It is the inverse of ShiftWorking, so a date reached by shifting n working
// days reports n.
func (c Calendar) WorkingDaysFromTo(from, to Date) int {
	if from.IsZero() || to.IsZero() || from == to {
		return 0
	}
	if to.After(from) {
		return c.WorkingDaysBetween(from.AddDays(1), to)
	}
	return -c.WorkingDaysBetween(to.AddDays(1), from)
}

// NextDay returns the day after a date on which a stage counted in the given
// basis may run.
func (c Calendar) NextDay(d Date, b Basis) Date {
	next := d.AddDays(1)
	if b.Normalise() == CalendarDays {
		return next
	}
	return c.OnOrAfterWorking(next)
}

// PrevDay returns the last day before a date on which a stage counted in the
// given basis may run.
func (c Calendar) PrevDay(d Date, b Basis) Date {
	prev := d.AddDays(-1)
	if b.Normalise() == CalendarDays {
		return prev
	}
	return c.OnOrBeforeWorking(prev)
}

// Snap moves a date forward, if it has to, onto a day a stage counted in the
// given basis may run on.
func (c Calendar) Snap(d Date, b Basis) Date {
	if b.Normalise() == CalendarDays {
		return d
	}
	return c.OnOrAfterWorking(d)
}

// SnapBack moves a date backward, if it has to, onto a day a stage counted in
// the given basis may run on.
func (c Calendar) SnapBack(d Date, b Basis) Date {
	if b.Normalise() == CalendarDays {
		return d
	}
	return c.OnOrBeforeWorking(d)
}

// SpanEnd is the day a stage finishes if it starts on a date and occupies the
// given number of days in the given basis. The count includes the first day, so
// a one-day stage finishes the day it starts.
func (c Calendar) SpanEnd(start Date, days int, b Basis) Date {
	if start.IsZero() {
		return start
	}
	if days < 1 {
		days = 1
	}
	if b.Normalise() == CalendarDays {
		return start.AddDays(days - 1)
	}
	return c.ShiftWorking(c.OnOrAfterWorking(start), days-1)
}

// SpanStart is the day a stage has to start if it must finish on a date and
// occupies the given number of days in the given basis. The count includes the
// finishing day.
func (c Calendar) SpanStart(finish Date, days int, b Basis) Date {
	if finish.IsZero() {
		return finish
	}
	if days < 1 {
		days = 1
	}
	if b.Normalise() == CalendarDays {
		return finish.AddDays(-(days - 1))
	}
	return c.ShiftWorking(c.OnOrBeforeWorking(finish), -(days - 1))
}

// Span returns the number of days of the given basis between two dates,
// counting both ends. It is how long a stage took.
func (c Calendar) Span(start, finish Date, b Basis) int {
	if start.IsZero() || finish.IsZero() || finish.Before(start) {
		return 0
	}
	if b.Normalise() == CalendarDays {
		return DaysBetween(start, finish) + 1
	}
	return c.WorkingDaysBetween(start, finish)
}

// Drift returns the movement from one date to another in the given basis:
// calendar days for a calendar stage, working days for a factory stage. It is
// the unit in which that stage's slack can actually be spent.
func (c Calendar) Drift(from, to Date, b Basis) int {
	if from.IsZero() || to.IsZero() {
		return 0
	}
	if b.Normalise() == CalendarDays {
		return DaysBetween(from, to)
	}
	return c.WorkingDaysFromTo(from, to)
}

// ClosedDaysBetween counts the days in a range on which the factory is shut,
// and names the holidays among them. This is the figure that explains why a
// twenty-working-day sewing run occupies twenty-six days on the wall calendar.
func (c Calendar) ClosedDaysBetween(from, to Date) (int, []Holiday) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return 0, nil
	}
	closed := 0
	var named []Holiday
	for d := from; !d.After(to); d = d.AddDays(1) {
		if c.IsWorkingDay(d) {
			continue
		}
		closed++
		if h, ok := c.HolidayOn(d); ok {
			named = append(named, h)
		}
	}
	return closed, named
}

// AddHoliday puts a day on the holiday list, replacing the name if that day is
// already there.
func (c Calendar) AddHoliday(h Holiday) (Calendar, error) {
	if h.On.IsZero() {
		return c, errors.New("a holiday needs a date")
	}
	h.Name = strings.TrimSpace(h.Name)
	if h.Name == "" {
		h.Name = "Holiday"
	}
	out := Calendar{WeeklyOff: c.WeeklyOff}
	replaced := false
	for _, existing := range c.Holidays {
		if existing.On == h.On {
			out.Holidays = append(out.Holidays, h)
			replaced = true
			continue
		}
		out.Holidays = append(out.Holidays, existing)
	}
	if !replaced {
		out.Holidays = append(out.Holidays, h)
	}
	return out.Normalise(), nil
}

// RemoveHoliday takes a day off the holiday list.
func (c Calendar) RemoveHoliday(on Date) (Calendar, error) {
	out := Calendar{WeeklyOff: c.WeeklyOff}
	found := false
	for _, existing := range c.Holidays {
		if existing.On == on {
			found = true
			continue
		}
		out.Holidays = append(out.Holidays, existing)
	}
	if !found {
		return c, fmt.Errorf("%s is not on the holiday list", on)
	}
	return out.Normalise(), nil
}
