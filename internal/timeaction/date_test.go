package timeaction

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Date
		wantErr bool
	}{
		{name: "ISO", in: "2026-06-30", want: NewDate(2026, time.June, 30)},
		{name: "day first with slashes", in: "30/06/2026", want: NewDate(2026, time.June, 30)},
		{name: "day first with dashes", in: "30-06-2026", want: NewDate(2026, time.June, 30)},
		{name: "day first with dots", in: "30.06.2026", want: NewDate(2026, time.June, 30)},
		{name: "single digit day and month", in: "1/4/2026", want: NewDate(2026, time.April, 1)},
		{name: "surrounding spaces", in: "  2026-06-30 ", want: NewDate(2026, time.June, 30)},
		{name: "empty reads as not given", in: "", want: Date{}},
		{name: "spaces only read as not given", in: "   ", want: Date{}},
		{name: "leap day in a leap year", in: "2024-02-29", want: NewDate(2024, time.February, 29)},
		{name: "leap day in a common year is refused", in: "2026-02-29", wantErr: true},
		{name: "month thirteen is refused", in: "2026-13-01", wantErr: true},
		{name: "words are refused", in: "next Tuesday", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDate(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseDate(%q) = %v, wanted an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseDate(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   Date
		want string
	}{
		{name: "a date", in: NewDate(2026, time.June, 30), want: `"2026-06-30"`},
		{name: "a leap day", in: NewDate(2024, time.February, 29), want: `"2024-02-29"`},
		{name: "never set", in: Date{}, want: `null`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, err := json.Marshal(c.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(body) != c.want {
				t.Errorf("marshalled %s, want %s", body, c.want)
			}

			var back Date
			if err := json.Unmarshal(body, &back); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if back != c.in {
				t.Errorf("round trip gave %v, want %v", back, c.in)
			}
		})
	}
}

func TestDateUnmarshalRejectsRubbish(t *testing.T) {
	cases := []string{`"the 30th"`, `17`, `{}`, `"2026-02-30"`}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var d Date
			if err := json.Unmarshal([]byte(in), &d); err == nil {
				t.Errorf("unmarshalling %s gave %v, wanted an error", in, d)
			}
		})
	}
}

func TestDateUnmarshalEmptyStringIsNotGiven(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`""`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !d.IsZero() {
		t.Errorf("empty string gave %v, want the zero date", d)
	}
}

func TestAddDays(t *testing.T) {
	cases := []struct {
		name string
		from Date
		n    int
		want Date
	}{
		{name: "within a month", from: NewDate(2026, time.June, 1), n: 5, want: NewDate(2026, time.June, 6)},
		{name: "over a month end", from: NewDate(2026, time.June, 30), n: 1, want: NewDate(2026, time.July, 1)},
		{name: "backwards over a month start", from: NewDate(2026, time.March, 1), n: -1, want: NewDate(2026, time.February, 28)},
		{name: "backwards over a leap month start", from: NewDate(2024, time.March, 1), n: -1, want: NewDate(2024, time.February, 29)},
		{name: "over the leap day", from: NewDate(2024, time.February, 28), n: 2, want: NewDate(2024, time.March, 1)},
		{name: "over a common February end", from: NewDate(2026, time.February, 28), n: 1, want: NewDate(2026, time.March, 1)},
		{name: "over a year end", from: NewDate(2026, time.December, 31), n: 1, want: NewDate(2027, time.January, 1)},
		{name: "a whole leap year", from: NewDate(2024, time.January, 1), n: 366, want: NewDate(2025, time.January, 1)},
		{name: "a whole common year", from: NewDate(2026, time.January, 1), n: 365, want: NewDate(2027, time.January, 1)},
		{name: "no movement", from: NewDate(2026, time.June, 30), n: 0, want: NewDate(2026, time.June, 30)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.from.AddDays(c.n); got != c.want {
				t.Errorf("%v.AddDays(%d) = %v, want %v", c.from, c.n, got, c.want)
			}
		})
	}
}

func TestAddDaysLeavesTheZeroDateAlone(t *testing.T) {
	if got := (Date{}).AddDays(10); !got.IsZero() {
		t.Errorf("moving the zero date gave %v, want the zero date", got)
	}
}

func TestDaysBetween(t *testing.T) {
	cases := []struct {
		name     string
		from, to Date
		want     int
	}{
		{name: "same day", from: NewDate(2026, time.June, 30), to: NewDate(2026, time.June, 30), want: 0},
		{name: "a week", from: NewDate(2026, time.June, 1), to: NewDate(2026, time.June, 8), want: 7},
		{name: "backwards", from: NewDate(2026, time.June, 8), to: NewDate(2026, time.June, 1), want: -7},
		{name: "across the leap day", from: NewDate(2024, time.February, 28), to: NewDate(2024, time.March, 1), want: 2},
		{name: "across a common February", from: NewDate(2026, time.February, 28), to: NewDate(2026, time.March, 1), want: 1},
		{name: "across a year end", from: NewDate(2026, time.December, 31), to: NewDate(2027, time.January, 1), want: 1},
		{name: "a leap year is 366 days", from: NewDate(2024, time.January, 1), to: NewDate(2025, time.January, 1), want: 366},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DaysBetween(c.from, c.to); got != c.want {
				t.Errorf("DaysBetween(%v, %v) = %d, want %d", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestDateFormatting(t *testing.T) {
	d := NewDate(2026, time.June, 30)
	if got, want := d.String(), "2026-06-30"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got, want := d.Long(), "Tue 30 Jun 2026"; got != want {
		t.Errorf("Long() = %q, want %q", got, want)
	}
	if got := (Date{}).String(); got != "" {
		t.Errorf("the zero date printed %q, want an empty string", got)
	}
	if got := (Date{}).Long(); got != "" {
		t.Errorf("the zero date printed long as %q, want an empty string", got)
	}
}

func TestNewDateNormalisesAnOverflowingDay(t *testing.T) {
	// 32 January is 1 February, the way time.Date reads it. A chain edited by
	// hand can produce this, and it must not become a date that does not exist.
	if got, want := NewDate(2026, time.January, 32), NewDate(2026, time.February, 1); got != want {
		t.Errorf("NewDate(2026, Jan, 32) = %v, want %v", got, want)
	}
}

func TestEarliestAndLatestIgnoreTheZeroDate(t *testing.T) {
	a := NewDate(2026, time.June, 1)
	b := NewDate(2026, time.June, 30)
	cases := []struct {
		name                 string
		x, y                 Date
		wantEarly, wantLater Date
	}{
		{name: "both given", x: a, y: b, wantEarly: a, wantLater: b},
		{name: "both given, reversed", x: b, y: a, wantEarly: a, wantLater: b},
		{name: "first not given", x: Date{}, y: b, wantEarly: b, wantLater: b},
		{name: "second not given", x: a, y: Date{}, wantEarly: a, wantLater: a},
		{name: "neither given", x: Date{}, y: Date{}, wantEarly: Date{}, wantLater: Date{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := earliest(c.x, c.y); got != c.wantEarly {
				t.Errorf("earliest(%v, %v) = %v, want %v", c.x, c.y, got, c.wantEarly)
			}
			if got := latest(c.x, c.y); got != c.wantLater {
				t.Errorf("latest(%v, %v) = %v, want %v", c.x, c.y, got, c.wantLater)
			}
		})
	}
}
