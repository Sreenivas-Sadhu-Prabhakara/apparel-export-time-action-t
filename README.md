# Apparel Export Time & Action Calendar

**Critical-path time-and-action calendar for garment export orders.** A single Go
binary that takes a buyer order's purchase-order date and ship date, schedules
every milestone from lab dip to on board **backwards from the ship date**, works
out the critical path and the slack on each milestone, and names the one
milestone that is threatening delivery today.

Each export order is a chain of deadlines — lab dip, fabric in-house, trims in,
cutting, sewing, finishing, final inspection, ex-factory, vessel cut-off, on
board — and one quiet slip at the front of it becomes an airfreight bill or a
cancelled order at the back.

## What it does

- **Backward scheduling from the ship date.** Enter the ship date and the lead
  times and every milestone date falls out of it, through your working week and
  your holiday list rather than through a calendar that pretends every day is a
  working day.
- **Critical path and slack.** The chain of milestones with no slack is
  *traced* — from the milestone that sets the earliest possible ship date, back
  through whatever is holding each one up — so it always comes out as one
  connected chain. Milestones off it get a slack figure in their own unit:
  working days for a factory stage, calendar days for transit.
- **"This order cannot be made on time."** Counted forwards from the
  purchase-order date, an order with nine weeks for a fourteen-week chain is not
  late — it was never possible. The tool says so immediately, and by how many
  days it is short.
- **The downstream knock-on, recomputed.** Record the date a milestone actually
  finished and everything behind it moves: finishing a week late pushes the
  inspection, the ex-factory handover, the inland transit and the vessel cut-off,
  and the projected on-board date moves with them.
- **Per-order status** — on track, at risk, late or shipped — driven by actual
  completion dates against planned ones, with the days the factory is shut taken
  into account.
- **A milestone chain per buyer.** The whole stage model is one named structure.
  Copy it, change the lead times and the approval loops a buyer actually works
  to, add a milestone they insist on, and nobody else's plan moves.
- **The whole order book on one board**, worst first, with the pieces at risk
  totalled — the figure that decides whether to book airfreight.
- **CSV out** — one order's time-and-action sheet, or the whole book.

## Who it's for

Export merchandisers and production planners in garment units in **India and the
Philippines** holding several buyer purchase orders at once: a Tiruppur knitwear
unit, a Noida woven house, a Cavite garment factory. It is for the person doing
the planning, not for an IT department.

It is not a production-planning or capacity system. It does arithmetic on dates.

## The rule it implements

**There is no regulation here, and this tool invents none.** No law sets a
sewing lead time and no authority publishes a time-and-action standard. What it
implements is ordinary **critical-path method** applied to the stage model a
garment export order runs through, with working-day arithmetic over a calendar
you control.

Three passes over the milestone chain:

| Pass | From | Gives |
| --- | --- | --- |
| Backward | the ship date | the **planned** date of every milestone — the last day it can run and still make the ship date |
| Forward | the purchase-order date | the **earliest** each milestone could run; the difference from the planned date is its float, and a forward pass landing past the ship date is the shortfall |
| Projection | the dates recorded as actually finished | where the order **now** lands, with nothing unfinished allowed to sit in the past |

A milestone must finish before its successors start; there is no overlap. The
critical path is traced rather than picked out by float, because a chain that
mixes working-day stages with calendar-day stages picks up a day here and there
as a span snaps over a closed day, and the milestones with the least float are
then not a connected chain at all.

**A backward plan has no spare days in it.** Every date is a last possible date,
so a milestone finishing a day after its own date moves the ship date — unless
the days the factory is shut happen to cover it, which the tool works out and
reports as *at risk* rather than *late*. The slack figure is the cushion the
order was **booked** with, not a reserve to spend at a milestone.

## Install and run

One binary, no dependencies, no installer. With Go 1.25 or newer:

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/apparel-export-time-action-t
cd apparel-export-time-action-t

# run it straight away
go run .

# or build the binary once and keep it
go build -o apparel-export-time-action-t .
./apparel-export-time-action-t
```

Then open <http://127.0.0.1:8080> in a browser on the same machine.

| Flag | Default | What it does |
| --- | --- | --- |
| `-addr` | `127.0.0.1:8080` | Address to listen on. Loopback only unless you deliberately change it. |
| `-data` | `./apparel-export-time-action-t-data.json` | Path to the local data file. |

Stop it with Ctrl-C.

## Where your data is stored

In one JSON file, wherever `-data` points: the order book, the milestone chains,
the working calendar and the completion dates you have recorded. It is plain
text you can open in any editor, and it is written atomically — to a temporary
file that is then renamed over the old one — so a crash or a full disk leaves
your previous good file intact rather than half a file. The file is created with
owner-only permissions, because an order book carries your buyers' names and
quantities.

That file is the whole system. Back it up by copying it; move it to another
machine by copying it. There is no account, no database, no sync and no cloud.
A file hand-edited into a state the tool cannot plan from is refused on the way
in, with the reason, rather than quietly producing dates that do not add up.

The binary makes **no outbound network connections of any kind**: no telemetry,
no analytics, no update check, no fonts or scripts fetched from anywhere, and no
connection to any buyer portal, forwarder system or shipping line. The one
socket it opens is the local one it serves the interface on.

## Indicative lead times

The chain the tool starts with has sixteen milestones. **Every lead time in it
is indicative** — trade practice for a mid-volume woven or knit programme, not a
rule, not anybody's published standard and not advice. All of them are named
constants in one file, `internal/timeaction/chain.go`, and all of them are
editable in the tool.

| Milestone | Lead | Counted in | Waits on |
| --- | --- | --- | --- |
| Purchase order placed | 1 | working days | — |
| Lab-dip and strike-off approved | 10 | working days | PO |
| Fabric booked with mill | 3 | working days | PO |
| Trims and packaging ordered | 3 | working days | PO |
| Fit sample approved | 12 | working days | PO |
| Fabric in-house | 35 | calendar days | fabric booked, lab dip |
| Trims in-house | 25 | calendar days | trims ordered |
| Pre-production sample approved | 7 | working days | fit sample, fabric in |
| Cutting | 4 | working days | fabric in, PP approval |
| Sewing | 18 | working days | cutting, trims in |
| Finishing, packing and cartoning | 6 | working days | sewing |
| Final inspection | 2 | working days | finishing |
| Ex-factory | 1 | working days | final inspection |
| Inland transit to port | 3 | calendar days | ex-factory |
| Vessel and documentation cut-off | 2 | calendar days | inland transit |
| On board | 1 | calendar days | vessel cut-off |

On a six-day week with no holidays that comes to **97 calendar days** from the
purchase order to the goods going on board.

Two more figures, in the same place and equally editable: the working week
starts as **one day off, Sunday, with no holidays at all**, and an order is
called at risk at **three days of slack or fewer**. No holiday list is shipped
on purpose — public holidays differ by country, by state and by district,
factories declare their own, and a wrong list is worse than an empty one because
it moves dates without anybody noticing.

## Scope and accuracy

**Replace every shipped lead time with your own before you plan against it.**
They are the commonest figures in the trade as a starting point, and they are
wrong for somebody. Take them from your own last few orders of that product for
that buyer. The vessel cut-off and the transit time come from your forwarder and
your shipping line and from nowhere else.

The tool cannot tell you whether a lead time you entered is achievable at your
own output, how many lines your unit has, or whether the mill will deliver on
the date they quoted. Give it wrong lead times and it will give you wrong dates,
confidently. Nothing here is checked against a buyer's portal, a forwarder's
booking system or a carrier's schedule, and no sailing date is verified.

This is a planning aid, not a production-planning system and not advice —
commercial, legal or professional.

## Development

```sh
gofmt -l .              # must print nothing
go vet ./...
go test ./... -count=1
go build -o /tmp/apparel-export-time-action-t-bin .
```

The rules live in `internal/timeaction` as pure functions over plain values,
with table-driven tests covering the working-day arithmetic across weekends,
runs of consecutive holidays, month ends, year ends and the leap day in both
directions; the backward and forward passes; the traced critical path; the
feasibility shortfall; and each status and knock-on case. The HTTP layer in
`main.go`, `store.go` and `api.go` holds no scheduling rules of its own.

MIT licensed.
