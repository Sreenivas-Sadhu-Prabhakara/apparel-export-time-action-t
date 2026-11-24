package timeaction

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Stage codes. Everything else in the tool refers to a milestone by its code,
// so these are the names in one place.
const (
	StagePO            = "PO"
	StageLabDip        = "LAB-DIP"
	StageFabricOrder   = "FABRIC-ORDER"
	StageTrimsOrder    = "TRIMS-ORDER"
	StageFitSample     = "FIT-SAMPLE"
	StageFabricIn      = "FABRIC-IN"
	StageTrimsIn       = "TRIMS-IN"
	StagePPApproval    = "PP-APPROVAL"
	StageCutting       = "CUTTING"
	StageSewing        = "SEWING"
	StageFinishing     = "FINISHING"
	StageInspection    = "INSPECTION"
	StageExFactory     = "EX-FACTORY"
	StageInlandTransit = "INLAND-TRANSIT"
	StageVesselCutOff  = "VESSEL-CUTOFF"
	StageOnBoard       = "ON-BOARD"
)

// Indicative lead times for the chain this build ships with, each in the basis
// its own stage is counted in.
//
// These are trade practice for a mid-volume woven or knit export programme, not
// rules and not anybody's published standard. They move with the buyer, the
// fabric, the season, the mill's order book and the shipping line, which is why
// every one of them is editable in the tool and why the shipped chain is
// labelled indicative wherever it is shown. Replace them with your own figures
// from your last few orders before you plan against them.
const (
	LeadPOPlacementDays           = 1  // working — confirming the order and opening the file
	LeadLabDipApprovalDays        = 10 // working — submit, buyer comments, resubmit
	LeadFabricBookingDays         = 3  // working — placing the order on the mill
	LeadTrimsOrderDays            = 3  // working — placing the trims and packaging orders
	LeadFitApprovalDays           = 12 // working — make the fit sample, courier it, get comments
	LeadFabricInHouseDays         = 35 // calendar — mill production, dispatch and transit
	LeadTrimsInHouseDays          = 25 // calendar — trims and packaging delivered to the unit
	LeadPreProductionApprovalDays = 7  // working — size set and pre-production approval
	LeadCuttingDays               = 4  // working
	LeadSewingDays                = 18 // working — the long pole on most orders
	LeadFinishingDays             = 6  // working — wash, press, trim, pack, carton
	LeadFinalInspectionDays       = 2  // working — final random inspection and any re-inspection
	LeadExFactoryDays             = 1  // working — handover to the forwarder
	LeadInlandTransitDays         = 3  // calendar — factory to port
	LeadVesselCutOffDays          = 2  // calendar — documentation and container yard cut-off
	LeadOnBoardDays               = 1  // calendar — loading and sailing
)

// DefaultChainName is what the shipped chain is called.
const DefaultChainName = "Standard export chain (indicative)"

// Stage is one milestone in the chain: a piece of work with a lead time, the
// milestones that must finish before it can start, and the person chased when
// it slips.
type Stage struct {
	Code  string   `json:"code"`
	Name  string   `json:"name"`
	Owner string   `json:"owner"`
	Days  int      `json:"days"`
	Basis Basis    `json:"basis"`
	After []string `json:"after"`
	Note  string   `json:"note"`
}

// Normalise tidies a stage: it puts the text and the codes into the one form
// the tool compares, and fills in a basis a file left blank. It deliberately
// does not repair a lead time or an unrecognised basis — those are figures the
// user meant something by, and Validate refuses them rather than quietly
// planning against a number nobody chose.
func (s Stage) Normalise() Stage {
	s.Code = NormaliseCode(s.Code)
	s.Name = strings.TrimSpace(s.Name)
	s.Owner = strings.TrimSpace(s.Owner)
	s.Note = strings.TrimSpace(s.Note)
	s.Basis = s.Basis.Normalise()
	if s.Name == "" {
		s.Name = s.Code
	}

	seen := map[string]bool{s.Code: true}
	var after []string
	for _, code := range s.After {
		code = NormaliseCode(code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		after = append(after, code)
	}
	s.After = after
	return s
}

// NormaliseCode puts a milestone code into the one form the tool compares, so
// that "sewing", "Sewing" and " SEWING " are the same milestone.
func NormaliseCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// Chain is the milestone chain for a buyer or a product: the whole time-and-
// action model in one named structure, so that a buyer who wants three
// approvals instead of two, or a mill that quotes sixty days instead of
// thirty-five, is a copy of this with the figures changed and not a change to
// the tool.
type Chain struct {
	Name   string  `json:"name"`
	Note   string  `json:"note"`
	Stages []Stage `json:"stages"`
}

// DefaultChain is the chain this build ships with: the sixteen milestones of a
// garment export order, from the buyer's purchase order to the goods going on
// board, with the indicative lead times above.
//
// Read the dependencies, not just the order. Fabric booking and the trims order
// can both start the day the purchase order lands, and the fit sample runs
// beside them; bulk fabric waits on the lab-dip approval; sewing waits on both
// cutting and the trims. That parallelism is what makes the slack figures worth
// anything — a flat list of stages one after another would show no float
// anywhere and would be wrong about which milestone matters.
func DefaultChain() Chain {
	return Chain{
		Name: DefaultChainName,
		Note: "Indicative lead times. Replace each one with your own figures for this buyer and this product.",
		Stages: []Stage{
			{Code: StagePO, Name: "Purchase order placed", Owner: "Merchandiser", Days: LeadPOPlacementDays, Basis: WorkingDays,
				Note: "Order confirmed, file opened, costing frozen."},
			{Code: StageLabDip, Name: "Lab-dip and strike-off approved", Owner: "Buyer", Days: LeadLabDipApprovalDays, Basis: WorkingDays, After: []string{StagePO},
				Note: "Bulk dyeing cannot start until the shade is signed off."},
			{Code: StageFabricOrder, Name: "Fabric booked with mill", Owner: "Merchandiser", Days: LeadFabricBookingDays, Basis: WorkingDays, After: []string{StagePO},
				Note: "Yarn and loom booking. Can run beside the lab dip."},
			{Code: StageTrimsOrder, Name: "Trims and packaging ordered", Owner: "Merchandiser", Days: LeadTrimsOrderDays, Basis: WorkingDays, After: []string{StagePO},
				Note: "Buttons, zips, labels, thread, polybags, cartons."},
			{Code: StageFitSample, Name: "Fit sample approved", Owner: "Buyer", Days: LeadFitApprovalDays, Basis: WorkingDays, After: []string{StagePO},
				Note: "Sample room, courier, buyer comments, resubmission."},
			{Code: StageFabricIn, Name: "Fabric in-house", Owner: "Mill", Days: LeadFabricInHouseDays, Basis: CalendarDays, After: []string{StageFabricOrder, StageLabDip},
				Note: "Mill production and inbound transit. Counted in calendar days: the mill does not keep your weekly off."},
			{Code: StageTrimsIn, Name: "Trims in-house", Owner: "Trims supplier", Days: LeadTrimsInHouseDays, Basis: CalendarDays, After: []string{StageTrimsOrder},
				Note: "Delivered and checked at the unit."},
			{Code: StagePPApproval, Name: "Pre-production sample approved", Owner: "Buyer", Days: LeadPreProductionApprovalDays, Basis: WorkingDays, After: []string{StageFitSample, StageFabricIn},
				Note: "Size set in bulk fabric, signed off before the bulk is cut."},
			{Code: StageCutting, Name: "Cutting", Owner: "Cutting room", Days: LeadCuttingDays, Basis: WorkingDays, After: []string{StageFabricIn, StagePPApproval}},
			{Code: StageSewing, Name: "Sewing", Owner: "Production", Days: LeadSewingDays, Basis: WorkingDays, After: []string{StageCutting, StageTrimsIn},
				Note: "The long pole on most orders."},
			{Code: StageFinishing, Name: "Finishing, packing and cartoning", Owner: "Finishing", Days: LeadFinishingDays, Basis: WorkingDays, After: []string{StageSewing}},
			{Code: StageInspection, Name: "Final inspection", Owner: "Buyer QA", Days: LeadFinalInspectionDays, Basis: WorkingDays, After: []string{StageFinishing},
				Note: "Final random inspection, with a day in hand for a re-inspection."},
			{Code: StageExFactory, Name: "Ex-factory", Owner: "Logistics", Days: LeadExFactoryDays, Basis: WorkingDays, After: []string{StageInspection},
				Note: "Goods handed to the forwarder."},
			{Code: StageInlandTransit, Name: "Inland transit to port", Owner: "Forwarder", Days: LeadInlandTransitDays, Basis: CalendarDays, After: []string{StageExFactory}},
			{Code: StageVesselCutOff, Name: "Vessel and documentation cut-off", Owner: "Forwarder", Days: LeadVesselCutOffDays, Basis: CalendarDays, After: []string{StageInlandTransit},
				Note: "Shipping instructions and container yard cut-off. Miss this and the booking rolls to the next sailing."},
			{Code: StageOnBoard, Name: "On board", Owner: "Carrier", Days: LeadOnBoardDays, Basis: CalendarDays, After: []string{StageVesselCutOff},
				Note: "The date the plan is scheduled backwards from."},
		},
	}
}

// Normalise tidies a chain.
func (ch Chain) Normalise() Chain {
	ch.Name = strings.TrimSpace(ch.Name)
	ch.Note = strings.TrimSpace(ch.Note)
	if ch.Name == "" {
		ch.Name = DefaultChainName
	}
	stages := make([]Stage, 0, len(ch.Stages))
	for _, s := range ch.Stages {
		stages = append(stages, s.Normalise())
	}
	ch.Stages = stages
	return ch
}

// Find returns a stage by code.
func (ch Chain) Find(code string) (Stage, bool) {
	code = NormaliseCode(code)
	for _, s := range ch.Stages {
		if s.Code == code {
			return s, true
		}
	}
	return Stage{}, false
}

// Codes lists every stage code in the order the chain holds them.
func (ch Chain) Codes() []string {
	out := make([]string, 0, len(ch.Stages))
	for _, s := range ch.Stages {
		out = append(out, s.Code)
	}
	return out
}

// Validate reports every problem with a chain at once, because a user editing
// sixteen milestones wants the whole list and not the first mistake.
func (ch Chain) Validate() error {
	var problems []string
	if len(ch.Stages) == 0 {
		return errors.New("a chain needs at least one milestone")
	}

	seen := map[string]bool{}
	for _, s := range ch.Stages {
		switch {
		case s.Code == "":
			problems = append(problems, "a milestone has no code")
		case seen[s.Code]:
			problems = append(problems, fmt.Sprintf("milestone %s appears twice", s.Code))
		default:
			seen[s.Code] = true
		}
		if s.Days < 1 {
			problems = append(problems, fmt.Sprintf("milestone %s has a lead time of %d days — a milestone takes at least one day", s.Code, s.Days))
		}
		if s.Basis != WorkingDays && s.Basis != CalendarDays {
			problems = append(problems, fmt.Sprintf("milestone %s is counted in %q, which is neither %q nor %q", s.Code, s.Basis, WorkingDays, CalendarDays))
		}
	}

	for _, s := range ch.Stages {
		for _, code := range s.After {
			if code == s.Code {
				problems = append(problems, fmt.Sprintf("milestone %s depends on itself", s.Code))
				continue
			}
			if !seen[code] {
				problems = append(problems, fmt.Sprintf("milestone %s depends on %s, which is not in this chain", s.Code, code))
			}
		}
	}

	if len(problems) == 0 {
		if _, err := ch.TopologicalOrder(); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// TopologicalOrder lists the stages so that every stage comes after everything
// it depends on. Among the stages that are ready at the same time it keeps the
// order the chain holds them in, so the plan reads the way the user arranged it
// rather than in whatever order a map happened to produce.
func (ch Chain) TopologicalOrder() ([]Stage, error) {
	remaining := make([]Stage, len(ch.Stages))
	copy(remaining, ch.Stages)

	done := map[string]bool{}
	out := make([]Stage, 0, len(remaining))

	for len(remaining) > 0 {
		progress := false
		for i, s := range remaining {
			ready := true
			for _, code := range s.After {
				// A dependency on a code that is not in the chain cannot be
				// waited for. Validate reports that separately; here it is
				// treated as satisfied so the sort still terminates.
				if _, ok := ch.Find(code); ok && !done[code] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			out = append(out, s)
			done[s.Code] = true
			remaining = append(remaining[:i], remaining[i+1:]...)
			progress = true
			break
		}
		if !progress {
			stuck := make([]string, 0, len(remaining))
			for _, s := range remaining {
				stuck = append(stuck, s.Code)
			}
			sort.Strings(stuck)
			return nil, fmt.Errorf("these milestones depend on each other in a circle, so no order of work exists: %s", strings.Join(stuck, ", "))
		}
	}
	return out, nil
}

// Successors maps each stage to the stages that wait on it.
func (ch Chain) Successors() map[string][]string {
	out := map[string][]string{}
	for _, s := range ch.Stages {
		for _, code := range s.After {
			out[code] = append(out[code], s.Code)
		}
	}
	return out
}

// Terminals lists the stages nothing waits on. These are the milestones the
// backward schedule anchors to the ship date; in the shipped chain there is one
// of them, the goods going on board.
func (ch Chain) Terminals() []string {
	successors := ch.Successors()
	var out []string
	for _, s := range ch.Stages {
		if len(successors[s.Code]) == 0 {
			out = append(out, s.Code)
		}
	}
	return out
}

// Roots lists the stages that wait on nothing. These start when the purchase
// order is placed.
func (ch Chain) Roots() []string {
	var out []string
	for _, s := range ch.Stages {
		if len(s.After) == 0 {
			out = append(out, s.Code)
		}
	}
	return out
}

// Clone returns a deep copy, so that editing one buyer's chain cannot reach
// into another's.
func (ch Chain) Clone() Chain {
	out := Chain{Name: ch.Name, Note: ch.Note, Stages: make([]Stage, 0, len(ch.Stages))}
	for _, s := range ch.Stages {
		copied := s
		copied.After = append([]string(nil), s.After...)
		out.Stages = append(out.Stages, copied)
	}
	return out
}

// TotalLeadDays adds up every stage's lead time in its own basis. It is not the
// length of the plan — stages run in parallel — but it is a useful check that
// an edited chain has not lost a figure.
func (ch Chain) TotalLeadDays() (working, calendar int) {
	for _, s := range ch.Stages {
		if s.Basis.Normalise() == CalendarDays {
			calendar += s.Days
			continue
		}
		working += s.Days
	}
	return working, calendar
}
