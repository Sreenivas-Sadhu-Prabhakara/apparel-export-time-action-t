package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/apparel-export-time-action-t/internal/timeaction"
)

// fileFormatVersion is written into the data file so a later version of the
// tool can tell what it is reading.
const fileFormatVersion = 1

// fileData is the whole contents of the data file: the working calendar, the
// thresholds the board is read by, the milestone chains, and the order book.
type fileData struct {
	Version  int                 `json:"version"`
	Calendar timeaction.Calendar `json:"calendar"`
	Settings timeaction.Settings `json:"settings"`
	Chains   []timeaction.Chain  `json:"chains"`
	Orders   []timeaction.Order  `json:"orders"`
}

// normalise fills in anything missing and puts everything into the one form the
// tool compares.
func (d fileData) normalise() fileData {
	d.Version = fileFormatVersion
	d.Calendar = d.Calendar.Normalise()
	if len(d.Calendar.WeeklyOff) == 0 && len(d.Calendar.Holidays) == 0 {
		d.Calendar = timeaction.DefaultCalendar()
	}
	d.Settings = d.Settings.Normalise()

	seen := map[string]bool{}
	chains := make([]timeaction.Chain, 0, len(d.Chains)+1)
	for _, ch := range d.Chains {
		ch = ch.Normalise()
		key := strings.ToLower(ch.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		chains = append(chains, ch)
	}
	if len(chains) == 0 {
		chains = append(chains, timeaction.DefaultChain().Normalise())
	}
	d.Chains = chains

	orders := make([]timeaction.Order, 0, len(d.Orders))
	for _, o := range d.Orders {
		o = o.Normalise()
		if o.ID == "" {
			o.ID = newID()
		}
		if o.Chain == "" {
			o.Chain = chains[0].Name
		}
		orders = append(orders, o)
	}
	d.Orders = orders
	return d
}

// store keeps the order book in memory and writes the whole file out after
// every change. An order book is a few hundred records at most, so rewriting
// the file is both simpler and safer than editing it in place.
type store struct {
	mu   sync.RWMutex
	path string
	data fileData
}

// openStore reads the data file, or starts an empty order book if there is no
// file yet. It does not create the file until something is saved.
func openStore(path string) (*store, error) {
	s := &store{path: path, data: fileData{
		Calendar: timeaction.DefaultCalendar(),
		Settings: timeaction.DefaultSettings(),
		Chains:   []timeaction.Chain{timeaction.DefaultChain()},
	}.normalise()}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return s, nil
	}

	var loaded fileData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("reading %s: %w (move the file aside to start again)", path, err)
	}
	if loaded.Version > fileFormatVersion {
		return nil, fmt.Errorf("%s was written by a newer version of this tool (format %d, this build reads %d)",
			path, loaded.Version, fileFormatVersion)
	}
	s.data = loaded.normalise()

	// A file edited by hand must not be allowed to produce plans that are
	// quietly wrong. Say so on the way in rather than on the way out.
	if err := s.data.Calendar.Validate(); err != nil {
		return nil, fmt.Errorf("the working calendar in %s cannot be used: %w", path, err)
	}
	for _, ch := range s.data.Chains {
		if err := ch.Validate(); err != nil {
			return nil, fmt.Errorf("the milestone chain %q in %s cannot be used:\n%w", ch.Name, path, err)
		}
	}
	return s, nil
}

// snapshot is a copy of the order book that a reader can use without holding
// the lock.
func (s *store) snapshot() fileData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.clone()
}

// clone copies the order book deeply, so that one caller cannot reach into
// another's copy through a shared slice or map.
func (d fileData) clone() fileData {
	out := d
	out.Calendar.WeeklyOff = append(out.Calendar.WeeklyOff[:0:0], d.Calendar.WeeklyOff...)
	out.Calendar.Holidays = append(out.Calendar.Holidays[:0:0], d.Calendar.Holidays...)

	out.Chains = make([]timeaction.Chain, 0, len(d.Chains))
	for _, ch := range d.Chains {
		out.Chains = append(out.Chains, ch.Clone())
	}

	out.Orders = make([]timeaction.Order, 0, len(d.Orders))
	for _, o := range d.Orders {
		copied := o
		copied.Actuals = map[string]timeaction.Date{}
		for code, on := range o.Actuals {
			copied.Actuals[code] = on
		}
		out.Orders = append(out.Orders, copied)
	}
	return out
}

// change applies a change and saves the file. If the change leaves the order
// book in a state the tool cannot use, or if saving fails, it is rolled back,
// so what is in memory always matches what is on disk.
func (s *store) change(apply func(d *fileData) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	before := s.data.clone()
	rollback := func() { s.data = before }

	if err := apply(&s.data); err != nil {
		rollback()
		return err
	}
	s.data = s.data.normalise()

	if err := s.data.Calendar.Validate(); err != nil {
		rollback()
		return err
	}
	for _, ch := range s.data.Chains {
		if err := ch.Validate(); err != nil {
			rollback()
			return fmt.Errorf("the milestone chain %q cannot be used:\n%w", ch.Name, err)
		}
	}
	if err := s.save(); err != nil {
		rollback()
		return err
	}
	return nil
}

// save writes the whole order book to the data file atomically: a temporary
// file in the same directory, flushed to disk, then renamed over the old one. A
// crash therefore leaves either the old file or the new one, never half of
// either.
func (s *store) save() error {
	s.data.Version = fileFormatVersion

	body, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("preparing the order book: %w", err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename has succeeded

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replacing %s: %w", s.path, err)
	}
	return nil
}

// newID makes an identity for an order.
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Only reached on a machine with no randomness at all.
		return fmt.Sprintf("o%d", len(b))
	}
	return hex.EncodeToString(b[:])
}

// chainNamed finds a chain by name, ignoring case and surrounding space.
func (d fileData) chainNamed(name string) (timeaction.Chain, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	for _, ch := range d.Chains {
		if strings.ToLower(ch.Name) == key {
			return ch, true
		}
	}
	return timeaction.Chain{}, false
}

// orderWithID finds an order and its position in the book.
func (d fileData) orderWithID(id string) (timeaction.Order, int, bool) {
	id = strings.TrimSpace(id)
	for i, o := range d.Orders {
		if o.ID == id {
			return o, i, true
		}
	}
	return timeaction.Order{}, -1, false
}

// plan works out one order's time and action.
func (d fileData) plan(o timeaction.Order, asOf timeaction.Date) (timeaction.Plan, error) {
	ch, ok := d.chainNamed(o.Chain)
	if !ok {
		return timeaction.Plan{}, fmt.Errorf("this order is planned on the chain %q, which is not in this file — add a chain of that name or move the order to one that exists", o.Chain)
	}
	return timeaction.Schedule(timeaction.Input{
		Chain:    ch,
		Calendar: d.Calendar,
		Settings: d.Settings,
		ShipDate: o.ShipDate,
		PODate:   o.PODate,
		Actuals:  o.Actuals,
		AsOf:     asOf,
	})
}

// board works the whole order book out at once.
func (d fileData) board(asOf timeaction.Date) timeaction.Board {
	rows := make([]timeaction.Row, 0, len(d.Orders))
	for _, o := range d.Orders {
		plan, err := d.plan(o, asOf)
		if err != nil {
			rows = append(rows, timeaction.FailedRow(o, err))
			continue
		}
		rows = append(rows, timeaction.BoardRow(o, plan))
	}
	return timeaction.BuildBoard(asOf, rows)
}

// putCalendar replaces the working calendar. Every date in every plan is worked
// out from it, so it is validated before it is kept.
func (s *store) putCalendar(next timeaction.Calendar) (timeaction.Calendar, error) {
	next = next.Normalise()
	if err := next.Validate(); err != nil {
		return next, err
	}
	err := s.change(func(d *fileData) error {
		d.Calendar = next
		return nil
	})
	return next, err
}

// addHoliday puts one day on the holiday list.
func (s *store) addHoliday(h timeaction.Holiday) (timeaction.Calendar, error) {
	var saved timeaction.Calendar
	err := s.change(func(d *fileData) error {
		next, err := d.Calendar.AddHoliday(h)
		if err != nil {
			return err
		}
		d.Calendar = next
		saved = next
		return nil
	})
	return saved, err
}

// removeHoliday takes one day off the holiday list.
func (s *store) removeHoliday(on timeaction.Date) (timeaction.Calendar, error) {
	var saved timeaction.Calendar
	err := s.change(func(d *fileData) error {
		next, err := d.Calendar.RemoveHoliday(on)
		if err != nil {
			return err
		}
		d.Calendar = next
		saved = next
		return nil
	})
	return saved, err
}

// putSettings replaces the thresholds the board is read by.
func (s *store) putSettings(next timeaction.Settings) (timeaction.Settings, error) {
	next = next.Normalise()
	err := s.change(func(d *fileData) error {
		d.Settings = next
		return nil
	})
	return next, err
}

// putChain adds a milestone chain, or replaces the one named by replacing.
//
// Naming the chain being replaced is what separates an edit from a copy: with
// no name given this adds a chain and refuses a name already taken, so that
// saving a buyer's variant can never overwrite somebody else's chain by
// accident. Renaming a chain carries the orders planned on it over, so they do
// not lose their plan.
func (s *store) putChain(next timeaction.Chain, replacing string) (timeaction.Chain, error) {
	next = next.Normalise()
	if err := next.Validate(); err != nil {
		return next, err
	}

	err := s.change(func(d *fileData) error {
		target := strings.ToLower(strings.TrimSpace(replacing))
		if target == "" {
			if _, clash := d.chainNamed(next.Name); clash {
				return fmt.Errorf("there is already a chain called %q — give this one another name, or save it over that one", next.Name)
			}
			d.Chains = append(d.Chains, next)
			return nil
		}

		at := -1
		for i, ch := range d.Chains {
			if strings.ToLower(ch.Name) == target {
				at = i
				break
			}
		}
		if at < 0 {
			return fmt.Errorf("there is no chain called %q to save this over", replacing)
		}

		oldName := d.Chains[at].Name
		if strings.ToLower(next.Name) != target {
			if _, clash := d.chainNamed(next.Name); clash {
				return fmt.Errorf("there is already a chain called %q", next.Name)
			}
			for i, o := range d.Orders {
				if strings.EqualFold(o.Chain, oldName) {
					d.Orders[i].Chain = next.Name
				}
			}
		}
		d.Chains[at] = next
		return nil
	})
	return next, err
}

// deleteChain removes a milestone chain. A chain with orders planned on it
// cannot be removed, because those orders would lose their plan, and the last
// chain cannot be removed either.
func (s *store) deleteChain(name string) error {
	return s.change(func(d *fileData) error {
		key := strings.ToLower(strings.TrimSpace(name))
		at := -1
		for i, ch := range d.Chains {
			if strings.ToLower(ch.Name) == key {
				at = i
				break
			}
		}
		if at < 0 {
			return fmt.Errorf("there is no chain called %q", name)
		}
		if len(d.Chains) == 1 {
			return errors.New("this is the only milestone chain in the file, and an order has to be planned on one — edit it instead of removing it")
		}

		var using int
		for _, o := range d.Orders {
			if strings.EqualFold(o.Chain, d.Chains[at].Name) {
				using++
			}
		}
		if using > 0 {
			return fmt.Errorf("%d order(s) are planned on %q — move them to another chain first", using, d.Chains[at].Name)
		}

		d.Chains = append(d.Chains[:at], d.Chains[at+1:]...)
		return nil
	})
}

// resetChain puts the shipped chain back, under its own name, leaving any other
// chain alone. It is the way back from an edit that went wrong, so it works
// whether that chain is still there or was renamed away.
func (s *store) resetChain() (timeaction.Chain, error) {
	shipped := timeaction.DefaultChain().Normalise()
	err := s.change(func(d *fileData) error {
		for i, ch := range d.Chains {
			if strings.EqualFold(ch.Name, shipped.Name) {
				d.Chains[i] = shipped
				return nil
			}
		}
		d.Chains = append(d.Chains, shipped)
		return nil
	})
	return shipped, err
}

// addOrder puts a new order in the book.
func (s *store) addOrder(o timeaction.Order) (timeaction.Order, error) {
	o = o.Normalise()
	if o.ID == "" {
		o.ID = newID()
	}
	if err := o.Validate(); err != nil {
		return o, err
	}

	err := s.change(func(d *fileData) error {
		if o.Chain == "" {
			o.Chain = d.Chains[0].Name
		}
		if _, ok := d.chainNamed(o.Chain); !ok {
			return fmt.Errorf("there is no milestone chain called %q", o.Chain)
		}
		if _, _, clash := d.orderWithID(o.ID); clash {
			return fmt.Errorf("an order with the identity %s is already in the book", o.ID)
		}
		d.Orders = append(d.Orders, o)
		return nil
	})
	return o, err
}

// putOrder replaces an order, keeping the completions already recorded against
// it unless the request carries its own.
func (s *store) putOrder(id string, next timeaction.Order) (timeaction.Order, error) {
	next = next.Normalise()
	if err := next.Validate(); err != nil {
		return next, err
	}

	var saved timeaction.Order
	err := s.change(func(d *fileData) error {
		existing, at, ok := d.orderWithID(id)
		if !ok {
			return fmt.Errorf("there is no order with the identity %s", id)
		}
		next.ID = existing.ID
		if len(next.Actuals) == 0 {
			next.Actuals = existing.Actuals
		}
		if next.Chain == "" {
			next.Chain = existing.Chain
		}
		if _, ok := d.chainNamed(next.Chain); !ok {
			return fmt.Errorf("there is no milestone chain called %q", next.Chain)
		}
		d.Orders[at] = next
		saved = next
		return nil
	})
	return saved, err
}

// deleteOrder takes an order out of the book.
func (s *store) deleteOrder(id string) error {
	return s.change(func(d *fileData) error {
		_, at, ok := d.orderWithID(id)
		if !ok {
			return fmt.Errorf("there is no order with the identity %s", id)
		}
		d.Orders = append(d.Orders[:at], d.Orders[at+1:]...)
		return nil
	})
}

// recordActual notes the date a milestone actually finished on.
func (s *store) recordActual(id, code string, on timeaction.Date) (timeaction.Order, error) {
	var saved timeaction.Order
	err := s.change(func(d *fileData) error {
		existing, at, ok := d.orderWithID(id)
		if !ok {
			return fmt.Errorf("there is no order with the identity %s", id)
		}
		ch, ok := d.chainNamed(existing.Chain)
		if !ok {
			return fmt.Errorf("this order is planned on the chain %q, which is not in this file", existing.Chain)
		}
		if _, ok := ch.Find(code); !ok {
			return fmt.Errorf("the chain %q has no milestone %s", ch.Name, timeaction.NormaliseCode(code))
		}

		next, err := existing.WithActual(code, on)
		if err != nil {
			return err
		}
		d.Orders[at] = next
		saved = next
		return nil
	})
	return saved, err
}

// clearActual removes a recorded completion, so a date typed wrongly can be
// taken back out.
func (s *store) clearActual(id, code string) (timeaction.Order, error) {
	var saved timeaction.Order
	err := s.change(func(d *fileData) error {
		existing, at, ok := d.orderWithID(id)
		if !ok {
			return fmt.Errorf("there is no order with the identity %s", id)
		}
		next, err := existing.WithoutActual(code)
		if err != nil {
			return err
		}
		d.Orders[at] = next
		saved = next
		return nil
	})
	return saved, err
}
