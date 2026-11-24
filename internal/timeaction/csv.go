package timeaction

import "strconv"

// PlanCSVHeader is the column heading of one order's time-and-action sheet.
func PlanCSVHeader() []string {
	return []string{
		"Milestone", "Code", "Owner", "Lead time", "Counted in",
		"Planned start", "Planned finish", "Actual finish",
		"Projected finish", "Slack", "Slack unit", "Critical path",
		"Status", "Days late", "Closed days in span", "Waiting on", "Note",
	}
}

// PlanCSVRows writes one order's plan out as a sheet, in the order the
// milestones are planned in.
func PlanCSVRows(p Plan) [][]string {
	rows := make([][]string, 0, len(p.Milestones))
	for _, m := range p.Milestones {
		late := m.LateDays
		if !m.Done {
			late = m.SlipDays
		}
		rows = append(rows, []string{
			m.Name,
			m.Code,
			m.Owner,
			strconv.Itoa(m.Days),
			m.Basis.Label(),
			m.PlannedStart.String(),
			m.PlannedFinish.String(),
			m.Actual.String(),
			m.ProjectedFinish.String(),
			strconv.Itoa(m.FloatDays),
			m.Basis.Label(),
			yesNo(m.Critical),
			m.Status.Label(),
			strconv.Itoa(late),
			strconv.Itoa(m.ClosedDays),
			m.DrivenBy,
			m.Note,
		})
	}
	return rows
}

// BoardCSVHeader is the column heading of the portfolio board.
func BoardCSVHeader() []string {
	return []string{
		"Buyer", "PO number", "Style", "Quantity", "Chain",
		"PO date", "Ship date", "Days to ship", "Status",
		"Projected on board", "Days late", "Short by", "Least slack (days)",
		"Chase", "Chase owner", "Chase due", "Milestones done", "Milestones overdue", "Verdict",
	}
}

// BoardCSVRows writes the board out in the order it is shown in.
func BoardCSVRows(b Board) [][]string {
	rows := make([][]string, 0, len(b.Rows))
	for _, r := range b.Rows {
		rows = append(rows, []string{
			r.Order.Buyer,
			r.Order.PONumber,
			r.Order.Style,
			strconv.Itoa(r.Order.Quantity),
			r.Order.Chain,
			r.Order.PODate.String(),
			r.ShipDate.String(),
			strconv.Itoa(r.DaysToShip),
			r.Status.Label(),
			r.ProjectedShipDate.String(),
			strconv.Itoa(r.SlipDays),
			strconv.Itoa(r.ShortfallDays),
			strconv.Itoa(r.MinFloatDays),
			r.ChaseName,
			r.ChaseOwner,
			r.ChaseDue.String(),
			strconv.Itoa(r.DoneCount) + " of " + strconv.Itoa(r.TotalCount),
			strconv.Itoa(r.Overdue),
			r.Verdict,
		})
	}
	return rows
}

// yesNo writes a flag the way a spreadsheet reads it.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
