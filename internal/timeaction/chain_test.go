package timeaction

import (
	"strings"
	"testing"
)

func TestDefaultChainMatchesTheShippedLeadTimes(t *testing.T) {
	// Every indicative lead time is a named constant in one place. This is the
	// table that proves the shipped chain is built from those constants and
	// from nothing else, so changing a constant changes the chain and a figure
	// cannot be left behind in two places.
	want := []struct {
		code  string
		days  int
		basis Basis
		after []string
	}{
		{StagePO, LeadPOPlacementDays, WorkingDays, nil},
		{StageLabDip, LeadLabDipApprovalDays, WorkingDays, []string{StagePO}},
		{StageFabricOrder, LeadFabricBookingDays, WorkingDays, []string{StagePO}},
		{StageTrimsOrder, LeadTrimsOrderDays, WorkingDays, []string{StagePO}},
		{StageFitSample, LeadFitApprovalDays, WorkingDays, []string{StagePO}},
		{StageFabricIn, LeadFabricInHouseDays, CalendarDays, []string{StageFabricOrder, StageLabDip}},
		{StageTrimsIn, LeadTrimsInHouseDays, CalendarDays, []string{StageTrimsOrder}},
		{StagePPApproval, LeadPreProductionApprovalDays, WorkingDays, []string{StageFitSample, StageFabricIn}},
		{StageCutting, LeadCuttingDays, WorkingDays, []string{StageFabricIn, StagePPApproval}},
		{StageSewing, LeadSewingDays, WorkingDays, []string{StageCutting, StageTrimsIn}},
		{StageFinishing, LeadFinishingDays, WorkingDays, []string{StageSewing}},
		{StageInspection, LeadFinalInspectionDays, WorkingDays, []string{StageFinishing}},
		{StageExFactory, LeadExFactoryDays, WorkingDays, []string{StageInspection}},
		{StageInlandTransit, LeadInlandTransitDays, CalendarDays, []string{StageExFactory}},
		{StageVesselCutOff, LeadVesselCutOffDays, CalendarDays, []string{StageInlandTransit}},
		{StageOnBoard, LeadOnBoardDays, CalendarDays, []string{StageVesselCutOff}},
	}

	ch := DefaultChain().Normalise()
	if len(ch.Stages) != len(want) {
		t.Fatalf("the shipped chain has %d milestones, want %d", len(ch.Stages), len(want))
	}

	for i, w := range want {
		got := ch.Stages[i]
		if got.Code != w.code {
			t.Errorf("milestone %d is %s, want %s", i, got.Code, w.code)
			continue
		}
		if got.Days != w.days {
			t.Errorf("%s lead time = %d, want %d", w.code, got.Days, w.days)
		}
		if got.Basis != w.basis {
			t.Errorf("%s is counted in %s, want %s", w.code, got.Basis, w.basis)
		}
		if len(got.After) != len(w.after) {
			t.Errorf("%s waits on %v, want %v", w.code, got.After, w.after)
			continue
		}
		for j := range w.after {
			if got.After[j] != w.after[j] {
				t.Errorf("%s waits on %v, want %v", w.code, got.After, w.after)
				break
			}
		}
		if got.Name == "" || got.Owner == "" {
			t.Errorf("%s has no name or no owner: a milestone nobody owns is never chased", w.code)
		}
	}
}

func TestDefaultChainShape(t *testing.T) {
	ch := DefaultChain().Normalise()
	if err := ch.Validate(); err != nil {
		t.Fatalf("the shipped chain does not validate: %v", err)
	}

	if roots := ch.Roots(); len(roots) != 1 || roots[0] != StagePO {
		t.Errorf("roots = %v, want only %s: everything starts from the purchase order", roots, StagePO)
	}
	if terminals := ch.Terminals(); len(terminals) != 1 || terminals[0] != StageOnBoard {
		t.Errorf("terminals = %v, want only %s: the ship date is what the plan hangs on", terminals, StageOnBoard)
	}

	working, calendar := ch.TotalLeadDays()
	if working <= 0 || calendar <= 0 {
		t.Errorf("total lead times = %d working and %d calendar days, want both above zero", working, calendar)
	}

	// Four milestones run beside the main run of work. If that parallelism were
	// lost the chain would be a straight line and every milestone would look
	// equally critical, which is the failure mode this tool exists to avoid.
	parallel := 0
	successors := ch.Successors()
	for _, s := range ch.Stages {
		if len(successors[s.Code]) > 1 {
			parallel++
		}
	}
	if parallel == 0 {
		t.Error("no milestone has more than one milestone waiting on it, so nothing runs in parallel")
	}
}

func TestChainValidate(t *testing.T) {
	cases := []struct {
		name     string
		chain    Chain
		wantErr  bool
		contains string
	}{
		{
			name:  "the shipped chain",
			chain: DefaultChain(),
		},
		{
			name:    "no milestones at all",
			chain:   Chain{Name: "Empty"},
			wantErr: true, contains: "at least one milestone",
		},
		{
			name: "the same code twice",
			chain: Chain{Stages: []Stage{
				{Code: "CUT", Days: 2}, {Code: "CUT", Days: 3},
			}},
			wantErr: true, contains: "appears twice",
		},
		{
			name: "a dependency that is not in the chain",
			chain: Chain{Stages: []Stage{
				{Code: "SEW", Days: 2, After: []string{"CUT"}},
			}},
			wantErr: true, contains: "not in this chain",
		},
		{
			name: "a milestone depending on itself",
			chain: Chain{Stages: []Stage{
				{Code: "SEW", Days: 2, After: []string{"SEW"}},
			}},
			wantErr: true, contains: "depends on itself",
		},
		{
			name: "two milestones waiting on each other",
			chain: Chain{Stages: []Stage{
				{Code: "A", Days: 1, Basis: WorkingDays, After: []string{"B"}},
				{Code: "B", Days: 1, Basis: WorkingDays, After: []string{"A"}},
			}},
			wantErr: true, contains: "circle",
		},
		{
			name: "a longer circle",
			chain: Chain{Stages: []Stage{
				{Code: "A", Days: 1, Basis: WorkingDays, After: []string{"C"}},
				{Code: "B", Days: 1, Basis: WorkingDays, After: []string{"A"}},
				{Code: "C", Days: 1, Basis: WorkingDays, After: []string{"B"}},
			}},
			wantErr: true, contains: "circle",
		},
		{
			name: "a lead time of zero days",
			chain: Chain{Stages: []Stage{
				{Code: "CUT", Days: 0, Basis: WorkingDays},
			}},
			wantErr: true, contains: "at least one day",
		},
		{
			name: "a basis that means nothing",
			chain: Chain{Stages: []Stage{
				{Code: "CUT", Days: 2, Basis: "fortnights"},
			}},
			wantErr: true, contains: "neither",
		},
		{
			name: "a milestone with no code",
			chain: Chain{Stages: []Stage{
				{Code: "   ", Days: 2, Basis: WorkingDays},
			}},
			wantErr: true, contains: "no code",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Validate is run on the raw chain, not a normalised one, because a
			// hand-edited file is exactly what it has to catch. Only the codes
			// are put into their comparison form.
			raw := c.chain
			for i := range raw.Stages {
				raw.Stages[i].Code = NormaliseCode(raw.Stages[i].Code)
			}
			err := raw.Validate()
			if !c.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("wanted an error, got none")
			}
			if c.contains != "" && !strings.Contains(err.Error(), c.contains) {
				t.Errorf("error %q does not mention %q", err, c.contains)
			}
		})
	}
}

func TestChainValidateReportsEveryProblemAtOnce(t *testing.T) {
	ch := Chain{Stages: []Stage{
		{Code: "A", Days: 0, Basis: WorkingDays},
		{Code: "A", Days: 1, Basis: WorkingDays},
		{Code: "B", Days: 1, Basis: "months", After: []string{"NOWHERE"}},
	}}
	err := ch.Validate()
	if err == nil {
		t.Fatal("wanted errors, got none")
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) < 4 {
		t.Errorf("reported %d problems, want at least 4:\n%s", len(lines), err)
	}
}

func TestTopologicalOrderPutsDependenciesFirst(t *testing.T) {
	ch := DefaultChain().Normalise()
	order, err := ch.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	if len(order) != len(ch.Stages) {
		t.Fatalf("ordered %d milestones, want %d", len(order), len(ch.Stages))
	}

	at := map[string]int{}
	for i, s := range order {
		at[s.Code] = i
	}
	for _, s := range ch.Stages {
		for _, pred := range s.After {
			if at[pred] >= at[s.Code] {
				t.Errorf("%s comes at %d but waits on %s at %d", s.Code, at[s.Code], pred, at[pred])
			}
		}
	}
}

func TestTopologicalOrderKeepsTheChainsOwnOrderAmongReadyMilestones(t *testing.T) {
	// Fabric booking, the trims order and the fit sample are all ready as soon
	// as the purchase order is placed. The sort must keep them in the order the
	// user arranged them, so the plan reads the same way twice.
	ch := DefaultChain().Normalise()
	for i := 0; i < 5; i++ {
		order, err := ch.TopologicalOrder()
		if err != nil {
			t.Fatalf("TopologicalOrder: %v", err)
		}
		var ready []string
		for _, s := range order {
			if len(s.After) == 1 && s.After[0] == StagePO {
				ready = append(ready, s.Code)
			}
		}
		want := []string{StageLabDip, StageFabricOrder, StageTrimsOrder, StageFitSample}
		if strings.Join(ready, ",") != strings.Join(want, ",") {
			t.Fatalf("run %d ordered the ready milestones %v, want %v", i, ready, want)
		}
	}
}

func TestStageNormalise(t *testing.T) {
	got := Stage{
		Code:  "  sewing  ",
		Name:  "   ",
		Owner: "  Production ",
		Days:  -4,
		Basis: "CALENDAR",
		After: []string{" cutting ", "CUTTING", "", "SEWING", "trims-in"},
		Note:  "  a note  ",
	}.Normalise()

	if got.Code != "SEWING" {
		t.Errorf("code = %q, want %q", got.Code, "SEWING")
	}
	if got.Name != "SEWING" {
		t.Errorf("a nameless milestone is called %q, want its code", got.Name)
	}
	if got.Owner != "Production" {
		t.Errorf("owner = %q, want it trimmed", got.Owner)
	}
	// A lead time is left exactly as the user gave it. Repairing it here would
	// plan against a figure nobody chose; Validate refuses it instead.
	if got.Days != -4 {
		t.Errorf("days = %d, want -4 left alone for Validate to refuse", got.Days)
	}
	if err := (Chain{Stages: []Stage{got}}).Validate(); err == nil {
		t.Error("a milestone with a negative lead time was accepted")
	}
	if got.Basis != CalendarDays {
		t.Errorf("basis = %q, want %q", got.Basis, CalendarDays)
	}
	if want := "CUTTING,TRIMS-IN"; strings.Join(got.After, ",") != want {
		t.Errorf("after = %v, want %v: duplicates, blanks and the self-reference removed", got.After, want)
	}
	if got.Note != "a note" {
		t.Errorf("note = %q, want it trimmed", got.Note)
	}
}

func TestChainNormaliseNamesAnUnnamedChain(t *testing.T) {
	got := Chain{Name: "   ", Stages: []Stage{{Code: "A", Days: 1}}}.Normalise()
	if got.Name != DefaultChainName {
		t.Errorf("name = %q, want %q", got.Name, DefaultChainName)
	}
}

func TestChainFindAndCodes(t *testing.T) {
	ch := DefaultChain().Normalise()
	if s, ok := ch.Find("  sewing "); !ok || s.Code != StageSewing {
		t.Errorf("Find(\"  sewing \") = %v, %v, want the sewing milestone", s.Code, ok)
	}
	if _, ok := ch.Find("EMBROIDERY"); ok {
		t.Error("found a milestone that is not in the chain")
	}
	if got, want := len(ch.Codes()), len(ch.Stages); got != want {
		t.Errorf("Codes() returned %d, want %d", got, want)
	}
}

func TestChainCloneIsIndependent(t *testing.T) {
	original := DefaultChain().Normalise()
	copied := original.Clone()

	copied.Name = "Changed"
	copied.Stages[0].Days = 99
	copied.Stages[1].After[0] = "ELSEWHERE"

	if original.Name == "Changed" {
		t.Error("renaming the copy renamed the original")
	}
	if original.Stages[0].Days == 99 {
		t.Error("changing a lead time on the copy changed the original")
	}
	if original.Stages[1].After[0] == "ELSEWHERE" {
		t.Error("changing a dependency on the copy changed the original")
	}
}

func TestTotalLeadDays(t *testing.T) {
	ch := Chain{Stages: []Stage{
		{Code: "A", Days: 3, Basis: WorkingDays},
		{Code: "B", Days: 5, Basis: CalendarDays},
		{Code: "C", Days: 7, Basis: WorkingDays},
	}}.Normalise()
	working, calendar := ch.TotalLeadDays()
	if working != 10 || calendar != 5 {
		t.Errorf("TotalLeadDays() = %d working, %d calendar; want 10 and 5", working, calendar)
	}
}

func TestNormaliseCode(t *testing.T) {
	cases := map[string]string{
		"sewing":      "SEWING",
		"  Sewing  ":  "SEWING",
		"TRIMS-IN":    "TRIMS-IN",
		"":            "",
		"   ":         "",
		"fabric-in ":  "FABRIC-IN",
		"Vessel-Cut ": "VESSEL-CUT",
	}
	for in, want := range cases {
		if got := NormaliseCode(in); got != want {
			t.Errorf("NormaliseCode(%q) = %q, want %q", in, got, want)
		}
	}
}
