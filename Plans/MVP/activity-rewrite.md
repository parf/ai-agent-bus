# Activity rewrite

📌 **TL;DR:** History. The accepted plan for [H.12](DONE.md#done--mvp): one
ring per record of 144 ten-minute slots on the node's clock, saved across
restarts. Built in 0.8.11–0.8.12; the current contract is [discovery § activity
history](../../docs/05-discovery.md#activity-history), which wins wherever this
file differs. The owner's later simplification is under [revision](#revision).

## Revision

2026-09-23, owner, before the build: a slot carries no state. A time nobody
counted — the daemon down, a skipped or daylight-saving hour — is simply zero,
and the open slot carries no partial mark. The Thesis, Rules, Faces and Steps
below are the plan as written; the rows marked *revised* no longer hold.

## Thesis

| | |
|---|---|
| Slot | Minute of the day ÷ 10, in node local time: slot 0 is 00:00–00:10, slot 143 is 23:50–24:00 |
| Ring | 144 slots per record. **End** is the open slot, **start** the oldest slot still inside the last day. Advancing end past slots nobody closed marks them unobserved; when end reaches start, start moves on |
| Value | Each slot holds the interval's In, Out, Dropped, Expired and Refused: at each boundary, the cumulative counter now minus the one at the previous boundary. Nothing is added to the send or consume path |
| Open slot | The current slot is read live, so the last ten minutes are always partial (*revised*: and carry no mark) |
| Scope | Per record. "All visible" is the sum of the rings the caller may see, as today |
| Durability | Closed slots are saved with the regular save batch and restored at start. Owner answer, 2026-09-23 |
| Clock | Node local time. Owner answer, 2026-09-23 |

## Existing and proposed

| | Existing (built 0.5.67) | Proposed |
|---|---|---|
| Structure | One daemon-wide slice of up to 145 samples, each a map of every record's counters (`src/internal/core/activity.go`); when full, every sample shifts down one | A fixed ring of 144 slots per record; start and end pointers, no shifting |
| Alignment | A ticker from process start (`src/cmd/agent-busd/bus.go`), so samples drift against the clock | Boundaries at :00, :10 … :50 of the node clock |
| Stored value | Cumulative counters, differenced on every read | The interval's own counts, differenced once at the boundary |
| Memory | 145 × every record's map entries, about 10–15 KB per record | 144 fixed slots, about 6 KB per record |
| Restart | History starts empty; a restart at 15:00 loses the morning | The day continues; the time the daemon was down is a run of zero slots (*revised*) |
| Partial interval | A live point appended to the samples | The open slot, read live (*revised*: no partial mark) |
| Reading | Under the bus lock, copy and difference the whole history | Sum the visible rings' slots |
| Web | One line per series, labelled only with the first and last time and 0/max; no gaps, because history never spans a restart | A fixed day axis with hour ticks; one continuous line (*revised*) |

## Rules

| Case | Rule |
|---|---|
| Unobserved and zero | *Revised:* the same answer, zero |
| Clock steps back | End never moves back; the counts go to the open slot |
| DST spring forward | The skipped hour leaves six zero slots |
| DST fall back | 01:00–01:50 comes twice; the second pass overwrites the first, a stated loss of one hour |
| Record removed | Its ring goes in the same commit, like every other stored reference to the name |
| Record transferred | Its ring stays: activity belongs to the name, not the Owner |
| Record inactive | Its ring is kept and served to nobody, as every read of an inactive record is |
| Restore | Slots older than a day are dropped; the gap between the last save and the start is zero |

## Persistence

Closed slots are written in the regular save batch, as credential last use is
(`FlushUsed`), never once per event. It takes a store schema bump with its
migration. The data model is written when the owner asks for it, per the
working rules.

## Faces

`/activity` and each record's sparkline read the day: a fixed 24-hour axis with
hour ticks and one continuous line (*revised*: no partial mark, no gaps).
The sample table lists each slot with its time. "All visible" keeps summing
what the caller may see.

## Found on the way

The Overview's call counters (`src/internal/callstats`) are sampled only on the
ten-minute activity tick, not every minute. So "Calls, minute" measures up to
ten minutes, and the 61 kept readings cover about ten hours rather than one.
That contradicts [what a node says about
itself](../../docs/05-discovery.md#what-a-node-says-about-itself). It is fixed
as its own step, by a minute tick.

## Steps

| Step | Acceptance |
|---|---|
| 1. Minute tick for call counters | "Calls, minute" covers one minute and 61 readings one hour; removing the minute tick fails its check |
| 2. Clock-aligned boundary | Every slot closes at :00, :10 … :50 local time whenever the daemon started; a start at 10:07 closes its first slot at 10:10 |
| 3. Ring and deltas | 144 slots per record with start and end; after a day the oldest slot is reused; a mutant that forgets to advance start, or to mark skipped slots, fails a test |
| 4. Persistence | A restart at 15:00 keeps 00:00–14:50; a daemon down from 12:00 to 13:00 shows six zero slots; removal deletes the ring in its commit |
| 5. Reading | "All visible" sums only visible records; an inactive record serves nothing; the open slot is read live |
| 6. Web | Fixed day axis with hour ticks; one continuous line where missing time is zero |
| 7. Docs | On acceptance the substance moves to [discovery § activity history](../../docs/05-discovery.md#activity-history), with a decision row, and this file becomes history |
