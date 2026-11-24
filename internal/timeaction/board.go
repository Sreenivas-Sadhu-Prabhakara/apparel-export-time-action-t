package timeaction

import "sort"

// Row is one order on the portfolio board: the whole order reduced to the few
// figures that decide whether it needs attention today.
type Row struct {
	Order Order  `json:"order"`
	Label string `json:"label"`

	Status   Status `json:"status"`
	Verdict  string `json:"verdict"`
	Complete bool   `json:"complete"`

	ShipDate   Date `json:"shipDate"`
	DaysToShip int  `json:"daysToShip"`

	ProjectedShipDate Date `json:"projectedShipDate"`
	SlipDays          int  `json:"slipDays"`
	ShortfallDays     int  `json:"shortfallDays"`
	Feasible          bool `json:"feasible"`
	MinFloatDays      int  `json:"minFloatDays"`

	ChaseCode  string `json:"chaseCode"`
	ChaseName  string `json:"chaseName"`
	ChaseOwner string `json:"chaseOwner"`
	ChaseDue   Date   `json:"chaseDue"`

	DoneCount  int      `json:"doneCount"`
	TotalCount int      `json:"totalCount"`
	Overdue    int      `json:"overdue"`
	Error      string   `json:"error"`
	Warnings   []string `json:"warnings"`
}

// Board is every order at once.
type Board struct {
	AsOf     Date  `json:"asOf"`
	Rows     []Row `json:"rows"`
	Late     int   `json:"late"`
	AtRisk   int   `json:"atRisk"`
	OnTrack  int   `json:"onTrack"`
	Complete int   `json:"complete"`
	Unusable int   `json:"unusable"`

	// Pieces is the quantity on the orders that are late or at risk, which is
	// the figure that decides whether to book airfreight.
	PiecesAtRisk int `json:"piecesAtRisk"`
	Pieces       int `json:"pieces"`
}

// BoardRow reduces one order and its plan to a board row.
func BoardRow(o Order, p Plan) Row {
	row := Row{
		Order:             o,
		Label:             o.Label(),
		Status:            p.Status,
		Verdict:           p.Verdict,
		Complete:          p.Complete,
		ShipDate:          p.ShipDate,
		DaysToShip:        p.DaysToShip,
		ProjectedShipDate: p.ProjectedShipDate,
		SlipDays:          p.SlipDays,
		ShortfallDays:     p.ShortfallDays,
		Feasible:          p.Feasible,
		MinFloatDays:      p.MinFloatDays,
		ChaseCode:         p.ChaseCode,
		ChaseName:         p.ChaseName,
		ChaseOwner:        p.ChaseOwner,
		ChaseDue:          p.ChaseDue,
		DoneCount:         p.DoneCount,
		TotalCount:        p.TotalCount,
		Warnings:          p.Warnings,
	}
	for _, m := range p.Milestones {
		if !m.Done && m.Overdue {
			row.Overdue++
		}
	}
	return row
}

// FailedRow is a board row for an order whose plan could not be worked out, so
// that a broken order is visible on the board rather than missing from it.
func FailedRow(o Order, err error) Row {
	row := Row{
		Order:    o,
		Label:    o.Label(),
		Status:   StatusLate,
		ShipDate: o.ShipDate,
		Error:    err.Error(),
		Verdict:  "This order cannot be planned: " + err.Error(),
	}
	return row
}

// BuildBoard gathers the rows, counts them and sorts them worst first: late
// orders at the top, then the ones at risk, then the ones on track, with
// completed orders last. Inside a group the order with the least slack comes
// first, then the one shipping soonest.
func BuildBoard(asOf Date, rows []Row) Board {
	if asOf.IsZero() {
		asOf = Today()
	}
	board := Board{AsOf: asOf, Rows: rows}

	for _, row := range rows {
		board.Pieces += row.Order.Quantity
		switch {
		case row.Error != "":
			board.Unusable++
		case row.Complete:
			board.Complete++
		case row.Status == StatusLate:
			board.Late++
		case row.Status == StatusAtRisk:
			board.AtRisk++
		default:
			board.OnTrack++
		}
		if row.Error == "" && !row.Complete && (row.Status == StatusLate || row.Status == StatusAtRisk) {
			board.PiecesAtRisk += row.Order.Quantity
		}
	}

	sort.SliceStable(board.Rows, func(i, j int) bool {
		a, b := board.Rows[i], board.Rows[j]
		if ra, rb := a.Status.Rank(), b.Status.Rank(); ra != rb {
			return ra < rb
		}
		if a.SlipDays != b.SlipDays {
			return a.SlipDays > b.SlipDays
		}
		if a.MinFloatDays != b.MinFloatDays {
			return a.MinFloatDays < b.MinFloatDays
		}
		if a.ShipDate != b.ShipDate {
			return a.ShipDate.Before(b.ShipDate)
		}
		return a.Label < b.Label
	})
	return board
}
