// Package timeaction works out the time-and-action plan for an apparel export
// order: the chain of milestones between the buyer's purchase order and the
// goods going on board, scheduled backwards from the ship date, with the
// critical path, the slack on every milestone, and what the dates become once
// actual completions are recorded against them.
//
// Everything here is a pure function over plain values. Nothing in this package
// reads a file, serves a request or looks at the clock, except Today.
package timeaction

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Date is a calendar day with no time and no zone. A ship date, a cut-off and a
// day on the factory floor are all whole days, and keeping them as whole days
// avoids the time-zone arithmetic that silently moves a deadline.
type Date struct {
	Y int
	M time.Month
	D int
}

// dateLayouts are the ways a date may be written: the ISO form the interface
// sends, and the day-first forms used on Indian and Philippine paperwork.
var dateLayouts = []string{"2006-01-02", "02/01/2006", "02-01-2006", "02.01.2006", "2/1/2006"}

// NewDate builds a date, normalising a day that overflows its month the way
// time.Date does.
func NewDate(y int, m time.Month, d int) Date {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return Date{Y: t.Year(), M: t.Month(), D: t.Day()}
}

// Today is the current date in the machine's own time zone.
func Today() Date {
	n := time.Now()
	return Date{Y: n.Year(), M: n.Month(), D: n.Day()}
}

// ParseDate reads a date written in any of the accepted forms. An empty string
// gives the zero date, which reads as "not given".
func ParseDate(s string) (Date, error) {
	text := strings.TrimSpace(s)
	if text == "" {
		return Date{}, nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, text); err == nil {
			return Date{Y: t.Year(), M: t.Month(), D: t.Day()}, nil
		}
	}
	return Date{}, fmt.Errorf("date %q is not a date — write it as 2026-04-30 or 30/04/2026", s)
}

// IsZero reports whether the date was never set.
func (d Date) IsZero() bool { return d.Y == 0 && d.M == 0 && d.D == 0 }

// Time returns the date as midnight UTC, for arithmetic and comparison.
func (d Date) Time() time.Time {
	return time.Date(d.Y, d.M, d.D, 0, 0, 0, 0, time.UTC)
}

// Weekday is the day of the week the date falls on.
func (d Date) Weekday() time.Weekday { return d.Time().Weekday() }

// String renders the date in ISO form, or an empty string when it was never set.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format("2006-01-02")
}

// Long renders the date the way a plan prints it: Thu 30 Apr 2026.
func (d Date) Long() string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format("Mon 2 Jan 2006")
}

// MarshalJSON writes the date as an ISO string, or null when it was never set.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON reads a date from an ISO string, null or an empty string.
func (d *Date) UnmarshalJSON(b []byte) error {
	text := strings.TrimSpace(string(b))
	if text == "null" || text == `""` {
		*d = Date{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("date %s did not read as a date", text)
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Before reports whether the date falls earlier than another.
func (d Date) Before(o Date) bool { return d.Time().Before(o.Time()) }

// After reports whether the date falls later than another.
func (d Date) After(o Date) bool { return d.Time().After(o.Time()) }

// AddDays moves the date forward by a number of calendar days, backward for a
// negative number.
func (d Date) AddDays(n int) Date {
	if d.IsZero() {
		return d
	}
	t := d.Time().AddDate(0, 0, n)
	return Date{Y: t.Year(), M: t.Month(), D: t.Day()}
}

// DaysBetween returns the number of whole calendar days from one date to
// another, negative when the second falls earlier.
func DaysBetween(from, to Date) int {
	return int(to.Time().Sub(from.Time()).Hours() / 24)
}

// earliest returns the earlier of two dates, ignoring a zero date.
func earliest(a, b Date) Date {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case b.Before(a):
		return b
	}
	return a
}

// latest returns the later of two dates, ignoring a zero date.
func latest(a, b Date) Date {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case b.After(a):
		return b
	}
	return a
}
