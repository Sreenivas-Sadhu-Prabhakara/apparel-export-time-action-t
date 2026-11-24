package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sreenivas-sadhu-prabhakara/apparel-export-time-action-t/internal/timeaction"
)

// api serves the JSON interface the page talks to. Every call is same-origin
// and nothing leaves the machine.
type api struct {
	store *store
}

// routes registers every endpoint.
func (a *api) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", a.getState)

	mux.HandleFunc("PUT /api/calendar", a.putCalendar)
	mux.HandleFunc("POST /api/holidays", a.postHoliday)
	mux.HandleFunc("DELETE /api/holidays", a.deleteHoliday)
	mux.HandleFunc("PUT /api/settings", a.putSettings)

	mux.HandleFunc("PUT /api/chains", a.putChain)
	mux.HandleFunc("DELETE /api/chains", a.deleteChain)
	mux.HandleFunc("POST /api/chains/reset", a.postResetChain)

	mux.HandleFunc("POST /api/orders", a.postOrder)
	mux.HandleFunc("PUT /api/orders", a.putOrder)
	mux.HandleFunc("DELETE /api/orders", a.deleteOrder)

	mux.HandleFunc("POST /api/orders/actuals", a.postActual)
	mux.HandleFunc("DELETE /api/orders/actuals", a.deleteActual)

	mux.HandleFunc("GET /api/orders/plan", a.getPlan)
	mux.HandleFunc("GET /api/orders/plan.csv", a.getPlanCSV)
	mux.HandleFunc("GET /api/board.csv", a.getBoardCSV)
}

// shippedDefaults are the figures this build starts with, sent to the page so
// that it can offer them and say where they came from rather than carry a
// second copy of them.
type shippedDefaults struct {
	Chain           timeaction.Chain    `json:"chain"`
	Calendar        timeaction.Calendar `json:"calendar"`
	Settings        timeaction.Settings `json:"settings"`
	AtRiskFloatDays int                 `json:"atRiskFloatDays"`
	Indicative      bool                `json:"indicative"`
}

// statePayload is everything the page needs to draw itself.
type statePayload struct {
	Today    timeaction.Date     `json:"today"`
	Calendar timeaction.Calendar `json:"calendar"`
	Settings timeaction.Settings `json:"settings"`
	Chains   []timeaction.Chain  `json:"chains"`
	Orders   []timeaction.Order  `json:"orders"`
	Board    timeaction.Board    `json:"board"`
	Plans    []planPayload       `json:"plans"`
	Defaults shippedDefaults     `json:"defaults"`
	DataFile string              `json:"dataFile"`
	Bases    []map[string]string `json:"bases"`
	Statuses []map[string]string `json:"statuses"`
	Weekdays []map[string]any    `json:"weekdays"`
	Totals   map[string]int      `json:"totals"`
	Problems map[string][]string `json:"problems"`
	Notes    map[string]string   `json:"notes"`
}

// planPayload is one order's worked-out plan, with the order it belongs to.
type planPayload struct {
	OrderID string          `json:"orderId"`
	Plan    timeaction.Plan `json:"plan"`
	Error   string          `json:"error"`
}

// getState answers with the whole order book as it stands.
func (a *api) getState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.state())
}

// state gathers everything the page draws itself from.
func (a *api) state() statePayload {
	data := a.store.snapshot()
	today := timeaction.Today()

	out := statePayload{
		Today:    today,
		Calendar: data.Calendar,
		Settings: data.Settings,
		Chains:   data.Chains,
		Orders:   data.Orders,
		Board:    data.board(today),
		DataFile: a.store.path,
		Defaults: shippedDefaults{
			Chain:           timeaction.DefaultChain().Normalise(),
			Calendar:        timeaction.DefaultCalendar(),
			Settings:        timeaction.DefaultSettings(),
			AtRiskFloatDays: timeaction.DefaultAtRiskFloatDays,
			Indicative:      true,
		},
		Bases: []map[string]string{
			{"value": string(timeaction.WorkingDays), "label": timeaction.WorkingDays.Label()},
			{"value": string(timeaction.CalendarDays), "label": timeaction.CalendarDays.Label()},
		},
		Statuses: []map[string]string{
			{"value": string(timeaction.StatusLate), "label": timeaction.StatusLate.Label()},
			{"value": string(timeaction.StatusAtRisk), "label": timeaction.StatusAtRisk.Label()},
			{"value": string(timeaction.StatusOnTrack), "label": timeaction.StatusOnTrack.Label()},
			{"value": string(timeaction.StatusDone), "label": timeaction.StatusDone.Label()},
		},
		Weekdays: weekdayList(),
		Totals:   map[string]int{},
		Problems: map[string][]string{},
		Notes: map[string]string{
			"storage":    "Everything stays in " + a.store.path + " on this machine. Nothing is sent anywhere.",
			"indicative": "The lead times this build starts with are trade practice, not rules. Replace each one with your own figures for this buyer and this product.",
		},
	}

	for _, o := range data.Orders {
		plan, err := data.plan(o, today)
		entry := planPayload{OrderID: o.ID, Plan: plan}
		if err != nil {
			entry.Error = err.Error()
			out.Problems[o.ID] = strings.Split(err.Error(), "\n")
		}
		out.Plans = append(out.Plans, entry)
	}

	for _, ch := range data.Chains {
		working, calendar := ch.TotalLeadDays()
		out.Totals[ch.Name+" · working"] = working
		out.Totals[ch.Name+" · calendar"] = calendar
	}
	return out
}

// weekdayList names the days of the week for the weekly-off control, in the
// numbering the calendar stores them in.
func weekdayList() []map[string]any {
	names := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	out := make([]map[string]any, 0, len(names))
	for i, name := range names {
		out = append(out, map[string]any{"value": i, "label": name, "short": name[:3]})
	}
	return out
}

// putCalendar saves the working week and the holiday list.
func (a *api) putCalendar(w http.ResponseWriter, r *http.Request) {
	var in timeaction.Calendar
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.putCalendar(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calendar": saved})
}

// postHoliday puts one day on the holiday list.
func (a *api) postHoliday(w http.ResponseWriter, r *http.Request) {
	var in timeaction.Holiday
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.addHoliday(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"calendar": saved})
}

// deleteHoliday takes one day off the holiday list.
func (a *api) deleteHoliday(w http.ResponseWriter, r *http.Request) {
	on, err := timeaction.ParseDate(r.URL.Query().Get("on"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	if on.IsZero() {
		writeProblem(w, http.StatusBadRequest, errors.New("which day? pass ?on=2026-08-15"))
		return
	}
	saved, err := a.store.removeHoliday(on)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calendar": saved})
}

// putSettings saves the thresholds the board is read by.
func (a *api) putSettings(w http.ResponseWriter, r *http.Request) {
	var in timeaction.Settings
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.putSettings(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": saved})
}

// putChain saves a milestone chain. The chain being replaced is named by
// ?replacing= when its own name is changing.
func (a *api) putChain(w http.ResponseWriter, r *http.Request) {
	var in timeaction.Chain
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.putChain(in, r.URL.Query().Get("replacing"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chain": saved})
}

// deleteChain removes a milestone chain.
func (a *api) deleteChain(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeProblem(w, http.StatusBadRequest, errors.New("which chain? pass ?name=<chain name>"))
		return
	}
	if err := a.store.deleteChain(name); err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": name})
}

// postResetChain puts the shipped chain back as it came.
func (a *api) postResetChain(w http.ResponseWriter, r *http.Request) {
	saved, err := a.store.resetChain()
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chain": saved})
}

// postOrder puts a new order in the book.
func (a *api) postOrder(w http.ResponseWriter, r *http.Request) {
	var in timeaction.Order
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.addOrder(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": saved})
}

// putOrder replaces an order.
func (a *api) putOrder(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeProblem(w, http.StatusBadRequest, errors.New("which order? pass ?id=<identity>"))
		return
	}
	var in timeaction.Order
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.putOrder(id, in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": saved})
}

// deleteOrder takes an order out of the book.
func (a *api) deleteOrder(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeProblem(w, http.StatusBadRequest, errors.New("which order? pass ?id=<identity>"))
		return
	}
	if err := a.store.deleteOrder(id); err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": id})
}

// completion is a recorded milestone finish, as the page sends it.
type completion struct {
	Code string          `json:"code"`
	On   timeaction.Date `json:"on"`
}

// postActual records the date a milestone actually finished on and answers with
// the order's recomputed plan, so the page can show the knock-on at once.
func (a *api) postActual(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeProblem(w, http.StatusBadRequest, errors.New("which order? pass ?id=<identity>"))
		return
	}
	var in completion
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.recordActual(id, in.Code, in.On)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	a.answerWithPlan(w, http.StatusOK, saved)
}

// deleteActual takes a recorded completion back out.
func (a *api) deleteActual(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if id == "" || code == "" {
		writeProblem(w, http.StatusBadRequest, errors.New("which completion? pass ?id=<identity>&code=<milestone>"))
		return
	}
	saved, err := a.store.clearActual(id, code)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	a.answerWithPlan(w, http.StatusOK, saved)
}

// getPlan answers with one order's worked-out plan.
func (a *api) getPlan(w http.ResponseWriter, r *http.Request) {
	order, err := a.order(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	a.answerWithPlan(w, http.StatusOK, order)
}

// answerWithPlan sends an order together with its recomputed plan.
func (a *api) answerWithPlan(w http.ResponseWriter, status int, order timeaction.Order) {
	data := a.store.snapshot()
	plan, err := data.plan(order, timeaction.Today())
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, status, map[string]any{"order": order, "plan": plan})
}

// order resolves the order a request asks for.
func (a *api) order(r *http.Request) (timeaction.Order, error) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		return timeaction.Order{}, errors.New("which order? pass ?id=<identity>")
	}
	data := a.store.snapshot()
	order, _, ok := data.orderWithID(id)
	if !ok {
		return timeaction.Order{}, fmt.Errorf("there is no order with the identity %s", id)
	}
	return order, nil
}

// getPlanCSV answers with one order's plan as a file, for the sheet that goes
// out to the factory and the buyer.
func (a *api) getPlanCSV(w http.ResponseWriter, r *http.Request) {
	order, err := a.order(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	data := a.store.snapshot()
	plan, err := data.plan(order, timeaction.Today())
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	name := "time-and-action"
	if order.PONumber != "" {
		name += "-" + safeFilePart(order.PONumber)
	} else if order.Buyer != "" {
		name += "-" + safeFilePart(order.Buyer)
	}
	writeCSV(w, name+".csv", timeaction.PlanCSVHeader(), timeaction.PlanCSVRows(plan))
}

// getBoardCSV answers with the whole order book as a file, worst first.
func (a *api) getBoardCSV(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	board := data.board(timeaction.Today())
	writeCSV(w, "order-book.csv", timeaction.BoardCSVHeader(), timeaction.BoardCSVRows(board))
}

// safeFilePart reduces a buyer or order number to something a file name can
// hold, so a download never carries a path separator.
func safeFilePart(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	trimmed := strings.Trim(b.String(), "-")
	if len(trimmed) > 48 {
		trimmed = trimmed[:48]
	}
	if trimmed == "" {
		return "order"
	}
	return trimmed
}

// readJSON reads a JSON request body, answering with a problem and reporting
// false when it cannot.
func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeProblem(w, http.StatusBadRequest, fmt.Errorf("this request did not read as JSON: %w", err))
		return false
	}
	return true
}

// writeJSON answers with a JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "could not prepare the answer", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeProblem answers with an error, splitting the several problems a form can
// have into a list the page can show against the fields.
func writeProblem(w http.ResponseWriter, status int, err error) {
	problems := strings.Split(err.Error(), "\n")
	writeJSON(w, status, map[string]any{
		"error":    strings.Join(problems, "; "),
		"problems": problems,
	})
}

// writeCSV answers with a CSV file for the browser to save.
func writeCSV(w http.ResponseWriter, filename string, header []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	out := csv.NewWriter(w)
	if err := out.Write(header); err != nil {
		return
	}
	for _, row := range rows {
		if err := out.Write(row); err != nil {
			return
		}
	}
	out.Flush()
}
