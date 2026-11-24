package timeaction

import (
	"testing"
	"time"
)

// sundayOff is the commonest garment-unit working week: six days on, Sunday off.
func sundayOff(holidays ...Holiday) Calendar {
	return Calendar{WeeklyOff: []time.Weekday{time.Sunday}, Holidays: holidays}.Normalise()
}

// diwaliShutdown is a four-day stretch with no work in it: a Sunday followed by
// three declared holidays. Consecutive closed days are where a plan quietly
// loses a week, so they are tested everywhere below.
func diwaliShutdown() Calendar {
	return sundayOff(
		Holiday{On: NewDate(2026, time.October, 19), Name: "Festival day 1"},
		Holiday{On: NewDate(2026, time.October, 20), Name: "Festival day 2"},
		Holiday{On: NewDate(2026, time.October, 21), Name: "Festival day 3"},
	)
}

func TestIsWorkingDay(t *testing.T) {
	cal := diwaliShutdown()
	cases := []struct {
		name string
		on   Date
		want bool
	}{
		{name: "a Saturday is worked", on: NewDate(2026, time.October, 17), want: true},
		{name: "the weekly off is not", on: NewDate(2026, time.October, 18), want: false},
		{name: "first holiday", on: NewDate(2026, time.October, 19), want: false},
		{name: "second holiday", on: NewDate(2026, time.October, 20), want: false},
		{name: "third holiday", on: NewDate(2026, time.October, 21), want: false},
		{name: "back to work", on: NewDate(2026, time.October, 22), want: true},
		{name: "the zero date is no day at all", on: Date{}, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cal.IsWorkingDay(c.on); got != c.want {
				t.Errorf("IsWorkingDay(%v) = %v, want %v", c.on, got, c.want)
			}
		})
	}
}

func TestHolidayOnTheWeeklyOffCostsOneDayNotTwo(t *testing.T) {
	// 1 February 2026 is a Sunday. Declaring a holiday on it must not remove a
	// second working day from the week.
	plain := sundayOff()
	doubled := sundayOff(Holiday{On: NewDate(2026, time.February, 1), Name: "Falls on the weekly off"})

	from, to := NewDate(2026, time.February, 1), NewDate(2026, time.February, 7)
	if got, want := plain.WorkingDaysBetween(from, to), 6; got != want {
		t.Fatalf("without the holiday the week has %d working days, want %d", got, want)
	}
	if got, want := doubled.WorkingDaysBetween(from, to), 6; got != want {
		t.Errorf("with a holiday on the weekly off the week has %d working days, want %d", got, want)
	}
}

func TestOnOrAfterAndBeforeWorking(t *testing.T) {
	cal := diwaliShutdown()
	cases := []struct {
		name              string
		on                Date
		wantAfter         Date
		wantBefore        Date
		skipBeforeCompare bool
	}{
		{
			name:       "already a working day",
			on:         NewDate(2026, time.October, 17),
			wantAfter:  NewDate(2026, time.October, 17),
			wantBefore: NewDate(2026, time.October, 17),
		},
		{
			name:       "the weekly off before a three day shutdown",
			on:         NewDate(2026, time.October, 18),
			wantAfter:  NewDate(2026, time.October, 22),
			wantBefore: NewDate(2026, time.October, 17),
		},
		{
			name:       "the middle of the shutdown",
			on:         NewDate(2026, time.October, 20),
			wantAfter:  NewDate(2026, time.October, 22),
			wantBefore: NewDate(2026, time.October, 17),
		},
		{
			name:       "the last closed day",
			on:         NewDate(2026, time.October, 21),
			wantAfter:  NewDate(2026, time.October, 22),
			wantBefore: NewDate(2026, time.October, 17),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cal.OnOrAfterWorking(c.on); got != c.wantAfter {
				t.Errorf("OnOrAfterWorking(%v) = %v, want %v", c.on, got, c.wantAfter)
			}
			if got := cal.OnOrBeforeWorking(c.on); got != c.wantBefore {
				t.Errorf("OnOrBeforeWorking(%v) = %v, want %v", c.on, got, c.wantBefore)
			}
		})
	}
}

func TestShiftWorking(t *testing.T) {
	leapClosed := sundayOff(Holiday{On: NewDate(2024, time.February, 29), Name: "Closed on the leap day"})

	cases := []struct {
		name string
		cal  Calendar
		from Date
		n    int
		want Date
	}{
		{
			name: "no movement leaves the day alone",
			cal:  sundayOff(), from: NewDate(2026, time.October, 18), n: 0,
			want: NewDate(2026, time.October, 18),
		},
		{
			name: "one working day over a weekly off",
			cal:  sundayOff(), from: NewDate(2026, time.February, 28), n: 1,
			want: NewDate(2026, time.March, 2),
		},
		{
			name: "one working day over a four day shutdown",
			cal:  diwaliShutdown(), from: NewDate(2026, time.October, 17), n: 1,
			want: NewDate(2026, time.October, 22),
		},
		{
			name: "two working days over a month end",
			cal:  sundayOff(), from: NewDate(2026, time.February, 27), n: 2,
			want: NewDate(2026, time.March, 2),
		},
		{
			name: "two working days over a month end with the Monday closed",
			cal:  sundayOff(Holiday{On: NewDate(2026, time.March, 2), Name: "Closed"}), from: NewDate(2026, time.February, 27), n: 2,
			want: NewDate(2026, time.March, 3),
		},
		{
			name: "two working days over the leap day",
			cal:  sundayOff(), from: NewDate(2024, time.February, 28), n: 2,
			want: NewDate(2024, time.March, 1),
		},
		{
			name: "two working days when the leap day is closed",
			cal:  leapClosed, from: NewDate(2024, time.February, 28), n: 2,
			want: NewDate(2024, time.March, 2),
		},
		{
			name: "six working days to the year end",
			cal:  sundayOff(), from: NewDate(2026, time.December, 24), n: 6,
			want: NewDate(2026, time.December, 31),
		},
		{
			name: "seven working days over the year end",
			cal:  sundayOff(), from: NewDate(2026, time.December, 24), n: 7,
			want: NewDate(2027, time.January, 1),
		},
		{
			name: "backwards over a four day shutdown",
			cal:  diwaliShutdown(), from: NewDate(2026, time.October, 22), n: -1,
			want: NewDate(2026, time.October, 17),
		},
		{
			name: "backwards over a month start",
			cal:  sundayOff(), from: NewDate(2026, time.March, 2), n: -1,
			want: NewDate(2026, time.February, 28),
		},
		{
			name: "backwards over the leap day",
			cal:  sundayOff(), from: NewDate(2024, time.March, 1), n: -2,
			want: NewDate(2024, time.February, 28),
		},
		{
			name: "a five day week skips both days off",
			cal:  Calendar{WeeklyOff: []time.Weekday{time.Saturday, time.Sunday}}.Normalise(),
			from: NewDate(2026, time.April, 3), n: 1,
			want: NewDate(2026, time.April, 6),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cal.ShiftWorking(c.from, c.n); got != c.want {
				t.Errorf("ShiftWorking(%v, %d) = %v, want %v", c.from, c.n, got, c.want)
			}
		})
	}
}

func TestShiftWorkingAndWorkingDaysFromToAreInverses(t *testing.T) {
	cal := diwaliShutdown()
	start := NewDate(2026, time.October, 15)
	for n := -25; n <= 25; n++ {
		landed := cal.ShiftWorking(start, n)
		if n != 0 && !cal.IsWorkingDay(landed) {
			t.Fatalf("ShiftWorking(%v, %d) landed on %v, which is not a working day", start, n, landed)
		}
		if got := cal.WorkingDaysFromTo(start, landed); got != n {
			t.Errorf("moved %d working days to %v but WorkingDaysFromTo reports %d", n, landed, got)
		}
	}
}

func TestWorkingDaysBetween(t *testing.T) {
	cases := []struct {
		name     string
		cal      Calendar
		from, to Date
		want     int
	}{
		{
			name: "one day", cal: sundayOff(),
			from: NewDate(2026, time.October, 22), to: NewDate(2026, time.October, 22), want: 1,
		},
		{
			name: "a closed day on its own counts nothing", cal: sundayOff(),
			from: NewDate(2026, time.October, 18), to: NewDate(2026, time.October, 18), want: 0,
		},
		{
			name: "a full week with one weekly off", cal: sundayOff(),
			from: NewDate(2026, time.October, 12), to: NewDate(2026, time.October, 18), want: 6,
		},
		{
			name: "a full week with a two day weekend",
			cal:  Calendar{WeeklyOff: []time.Weekday{time.Saturday, time.Sunday}}.Normalise(),
			from: NewDate(2026, time.October, 12), to: NewDate(2026, time.October, 18), want: 5,
		},
		{
			name: "across a four day shutdown", cal: diwaliShutdown(),
			from: NewDate(2026, time.October, 17), to: NewDate(2026, time.October, 22), want: 2,
		},
		{
			name: "across a leap month boundary", cal: sundayOff(),
			from: NewDate(2024, time.February, 26), to: NewDate(2024, time.March, 1), want: 5,
		},
		{
			name: "across a common February boundary", cal: sundayOff(),
			from: NewDate(2026, time.February, 27), to: NewDate(2026, time.March, 2), want: 3,
		},
		{
			name: "a whole leap February", cal: sundayOff(),
			from: NewDate(2024, time.February, 1), to: NewDate(2024, time.February, 29), want: 25,
		},
		{
			name: "a whole common February", cal: sundayOff(),
			from: NewDate(2026, time.February, 1), to: NewDate(2026, time.February, 28), want: 24,
		},
		{
			name: "a whole leap February on a five day week",
			cal:  Calendar{WeeklyOff: []time.Weekday{time.Saturday, time.Sunday}}.Normalise(),
			from: NewDate(2024, time.February, 1), to: NewDate(2024, time.February, 29), want: 21,
		},
		{
			name: "across a year end", cal: sundayOff(),
			from: NewDate(2026, time.December, 31), to: NewDate(2027, time.January, 1), want: 2,
		},
		{
			name: "an inverted range counts nothing", cal: sundayOff(),
			from: NewDate(2026, time.October, 22), to: NewDate(2026, time.October, 17), want: 0,
		},
		{
			name: "a range with no date counts nothing", cal: sundayOff(),
			from: Date{}, to: NewDate(2026, time.October, 22), want: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cal.WorkingDaysBetween(c.from, c.to); got != c.want {
				t.Errorf("WorkingDaysBetween(%v, %v) = %d, want %d", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestWorkingDaysFromTo(t *testing.T) {
	cal := diwaliShutdown()
	cases := []struct {
		name     string
		from, to Date
		want     int
	}{
		{name: "the same day is no movement", from: NewDate(2026, time.October, 17), to: NewDate(2026, time.October, 17), want: 0},
		{name: "over a four day shutdown", from: NewDate(2026, time.October, 17), to: NewDate(2026, time.October, 22), want: 1},
		{name: "backwards over a four day shutdown", from: NewDate(2026, time.October, 22), to: NewDate(2026, time.October, 17), want: -1},
		{name: "a working week forward", from: NewDate(2026, time.October, 22), to: NewDate(2026, time.October, 28), want: 5},
		{name: "a working week back", from: NewDate(2026, time.October, 28), to: NewDate(2026, time.October, 22), want: -5},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cal.WorkingDaysFromTo(c.from, c.to); got != c.want {
				t.Errorf("WorkingDaysFromTo(%v, %v) = %d, want %d", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestSpanEndAndSpanStart(t *testing.T) {
	cal := diwaliShutdown()
	cases := []struct {
		name       string
		basis      Basis
		start      Date
		days       int
		wantFinish Date
	}{
		{name: "a one day working stage finishes the day it starts", basis: WorkingDays,
			start: NewDate(2026, time.October, 17), days: 1, wantFinish: NewDate(2026, time.October, 17)},
		{name: "a two day working stage steps over the shutdown", basis: WorkingDays,
			start: NewDate(2026, time.October, 17), days: 2, wantFinish: NewDate(2026, time.October, 22)},
		{name: "a six day working stage over the shutdown", basis: WorkingDays,
			start: NewDate(2026, time.October, 17), days: 6, wantFinish: NewDate(2026, time.October, 27)},
		{name: "a calendar stage ignores the shutdown", basis: CalendarDays,
			start: NewDate(2026, time.October, 17), days: 6, wantFinish: NewDate(2026, time.October, 22)},
		{name: "a one day calendar stage finishes the day it starts", basis: CalendarDays,
			start: NewDate(2026, time.October, 18), days: 1, wantFinish: NewDate(2026, time.October, 18)},
		{name: "a working stage starting on a closed day is pushed to the next open one", basis: WorkingDays,
			start: NewDate(2026, time.October, 18), days: 1, wantFinish: NewDate(2026, time.October, 22)},
		{name: "a working stage over a leap month end", basis: WorkingDays,
			start: NewDate(2024, time.February, 26), days: 5, wantFinish: NewDate(2024, time.March, 1)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cal.SpanEnd(c.start, c.days, c.basis)
			if got != c.wantFinish {
				t.Fatalf("SpanEnd(%v, %d, %s) = %v, want %v", c.start, c.days, c.basis, got, c.wantFinish)
			}
			// Going back from the finish must land on the day the stage was
			// actually able to start, which is the start snapped onto an open day.
			wantStart := cal.Snap(c.start, c.basis)
			if back := cal.SpanStart(got, c.days, c.basis); back != wantStart {
				t.Errorf("SpanStart(%v, %d, %s) = %v, want %v", got, c.days, c.basis, back, wantStart)
			}
			if span := cal.Span(wantStart, got, c.basis); span != c.days {
				t.Errorf("the span %v..%v measures %d %s, want %d", wantStart, got, span, c.basis.Label(), c.days)
			}
		})
	}
}

func TestSpanTreatsADayCountBelowOneAsOneDay(t *testing.T) {
	cal := sundayOff()
	day := NewDate(2026, time.October, 22)
	for _, days := range []int{0, -3} {
		if got := cal.SpanEnd(day, days, WorkingDays); got != day {
			t.Errorf("SpanEnd(%v, %d) = %v, want %v", day, days, got, day)
		}
		if got := cal.SpanStart(day, days, CalendarDays); got != day {
			t.Errorf("SpanStart(%v, %d) = %v, want %v", day, days, got, day)
		}
	}
}

func TestDrift(t *testing.T) {
	cal := sundayOff()
	// Friday to Monday is three calendar days but, on a six-day week, two
	// working days: a factory stage with that much slack can slip two shifts,
	// not three. Reporting slack in the wrong unit is how a plan is overspent.
	friday := NewDate(2026, time.February, 27)
	monday := NewDate(2026, time.March, 2)

	if got, want := cal.Drift(friday, monday, CalendarDays), 3; got != want {
		t.Errorf("calendar drift = %d, want %d", got, want)
	}
	if got, want := cal.Drift(friday, monday, WorkingDays), 2; got != want {
		t.Errorf("working drift = %d, want %d", got, want)
	}
	if got, want := cal.Drift(monday, friday, WorkingDays), -2; got != want {
		t.Errorf("backward working drift = %d, want %d", got, want)
	}
	if got, want := cal.Drift(Date{}, monday, WorkingDays), 0; got != want {
		t.Errorf("drift from no date = %d, want %d", got, want)
	}
}

func TestClosedDaysBetween(t *testing.T) {
	cal := diwaliShutdown()
	closed, named := cal.ClosedDaysBetween(NewDate(2026, time.October, 17), NewDate(2026, time.October, 22))
	if want := 4; closed != want {
		t.Errorf("closed days = %d, want %d", closed, want)
	}
	if want := 3; len(named) != want {
		t.Fatalf("named holidays = %d, want %d", len(named), want)
	}
	if got, want := named[0].Name, "Festival day 1"; got != want {
		t.Errorf("first named holiday = %q, want %q", got, want)
	}

	if closed, named := cal.ClosedDaysBetween(NewDate(2026, time.October, 22), NewDate(2026, time.October, 17)); closed != 0 || named != nil {
		t.Errorf("an inverted range gave %d closed days and %v", closed, named)
	}
}

func TestCalendarNormalise(t *testing.T) {
	messy := Calendar{
		WeeklyOff: []time.Weekday{time.Sunday, time.Sunday, time.Weekday(9), time.Saturday},
		Holidays: []Holiday{
			{On: NewDate(2026, time.December, 25), Name: "  Christmas  "},
			{On: NewDate(2026, time.January, 26), Name: "New year"},
			{On: NewDate(2026, time.December, 25), Name: "Entered twice"},
			{On: Date{}, Name: "No date at all"},
			{On: NewDate(2026, time.August, 15), Name: ""},
		},
	}
	got := messy.Normalise()

	// Go numbers the weekdays from Sunday, so sorted order is Sunday first.
	if want := []time.Weekday{time.Sunday, time.Saturday}; len(got.WeeklyOff) != len(want) {
		t.Fatalf("weekly off = %v, want %v", got.WeeklyOff, want)
	}
	if got.WeeklyOff[0] != time.Sunday || got.WeeklyOff[1] != time.Saturday {
		t.Errorf("weekly off = %v, want it sorted and de-duplicated", got.WeeklyOff)
	}
	if len(got.Holidays) != 3 {
		t.Fatalf("holidays = %v, want 3 of them", got.Holidays)
	}
	if !got.Holidays[0].On.Before(got.Holidays[1].On) || !got.Holidays[1].On.Before(got.Holidays[2].On) {
		t.Errorf("holidays are not in date order: %v", got.Holidays)
	}
	if name := got.Holidays[2].Name; name != "Christmas" {
		t.Errorf("the name %q was not trimmed, and the first entry should win over the duplicate", name)
	}
	if name := got.Holidays[1].Name; name != "Holiday" {
		t.Errorf("a nameless holiday is called %q, want %q", name, "Holiday")
	}
	if got.WorkingWeekDays() != 5 {
		t.Errorf("working week = %d days, want 5", got.WorkingWeekDays())
	}
}

func TestCalendarValidate(t *testing.T) {
	cases := []struct {
		name    string
		cal     Calendar
		wantErr bool
	}{
		{name: "the shipped calendar", cal: DefaultCalendar()},
		{name: "six days off still leaves a working day", cal: Calendar{WeeklyOff: []time.Weekday{
			time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}}},
		{name: "every day off", wantErr: true, cal: Calendar{WeeklyOff: []time.Weekday{
			time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}}},
		{name: "a holiday with no date", wantErr: true, cal: Calendar{
			WeeklyOff: []time.Weekday{time.Sunday},
			Holidays:  []Holiday{{Name: "When?"}},
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cal.Validate()
			if c.wantErr && err == nil {
				t.Error("wanted an error, got none")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultCalendarIsOneDayOffAndNoHolidays(t *testing.T) {
	cal := DefaultCalendar()
	if len(cal.WeeklyOff) != 1 || cal.WeeklyOff[0] != time.Sunday {
		t.Errorf("weekly off = %v, want Sunday only", cal.WeeklyOff)
	}
	// Holidays differ by country, by state and by factory. Shipping a list
	// would move dates the user never agreed to.
	if len(cal.Holidays) != 0 {
		t.Errorf("holidays = %v, want none shipped", cal.Holidays)
	}
}

func TestAddAndRemoveHoliday(t *testing.T) {
	cal := sundayOff()
	day := NewDate(2026, time.August, 15)

	added, err := cal.AddHoliday(Holiday{On: day, Name: " Independence Day "})
	if err != nil {
		t.Fatalf("AddHoliday: %v", err)
	}
	if len(added.Holidays) != 1 || added.Holidays[0].Name != "Independence Day" {
		t.Fatalf("holidays = %v, want one trimmed entry", added.Holidays)
	}
	if added.IsWorkingDay(day) {
		t.Error("the day is still a working day after being declared a holiday")
	}
	if len(cal.Holidays) != 0 {
		t.Error("adding a holiday changed the calendar it was added to")
	}

	renamed, err := added.AddHoliday(Holiday{On: day, Name: "Renamed"})
	if err != nil {
		t.Fatalf("AddHoliday again: %v", err)
	}
	if len(renamed.Holidays) != 1 || renamed.Holidays[0].Name != "Renamed" {
		t.Errorf("holidays = %v, want the one entry renamed", renamed.Holidays)
	}

	if _, err := cal.AddHoliday(Holiday{Name: "No date"}); err == nil {
		t.Error("a holiday with no date was accepted")
	}

	gone, err := renamed.RemoveHoliday(day)
	if err != nil {
		t.Fatalf("RemoveHoliday: %v", err)
	}
	if len(gone.Holidays) != 0 {
		t.Errorf("holidays = %v, want none", gone.Holidays)
	}
	if _, err := gone.RemoveHoliday(day); err == nil {
		t.Error("removing a day that is not on the list was accepted")
	}
}

func TestBasisNormaliseAndLabel(t *testing.T) {
	cases := []struct {
		in        Basis
		want      Basis
		wantLabel string
	}{
		{in: WorkingDays, want: WorkingDays, wantLabel: "working days"},
		{in: CalendarDays, want: CalendarDays, wantLabel: "calendar days"},
		{in: "CALENDAR", want: CalendarDays, wantLabel: "calendar days"},
		{in: "  calendar ", want: CalendarDays, wantLabel: "calendar days"},
		{in: "", want: WorkingDays, wantLabel: "working days"},
		// A value that is neither is left as it was found, so that a chain
		// carrying it is refused rather than quietly read as working days.
		{in: "fortnights", want: "fortnights", wantLabel: "working days"},
		{in: "  Fortnights ", want: "fortnights", wantLabel: "working days"},
	}
	for _, c := range cases {
		t.Run(string(c.in), func(t *testing.T) {
			if got := c.in.Normalise(); got != c.want {
				t.Errorf("Normalise() = %q, want %q", got, c.want)
			}
			if got := c.in.Label(); got != c.wantLabel {
				t.Errorf("Label() = %q, want %q", got, c.wantLabel)
			}
		})
	}
}

func TestNextAndPrevDayRespectTheBasis(t *testing.T) {
	cal := diwaliShutdown()
	saturday := NewDate(2026, time.October, 17)

	if got, want := cal.NextDay(saturday, WorkingDays), NewDate(2026, time.October, 22); got != want {
		t.Errorf("next working day after %v = %v, want %v", saturday, got, want)
	}
	if got, want := cal.NextDay(saturday, CalendarDays), NewDate(2026, time.October, 18); got != want {
		t.Errorf("next calendar day after %v = %v, want %v", saturday, got, want)
	}

	thursday := NewDate(2026, time.October, 22)
	if got, want := cal.PrevDay(thursday, WorkingDays), saturday; got != want {
		t.Errorf("last working day before %v = %v, want %v", thursday, got, want)
	}
	if got, want := cal.PrevDay(thursday, CalendarDays), NewDate(2026, time.October, 21); got != want {
		t.Errorf("last calendar day before %v = %v, want %v", thursday, got, want)
	}
}
