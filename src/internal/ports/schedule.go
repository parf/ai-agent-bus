package ports

// The schedule port: each record's schedules, kept in the daemon's database
// beside its key-value store. See Plans/R1.0-Release/cron-in-core.md#storage.

// Schedule is one stored schedule. Times are UTC unix seconds; zero is none.
type Schedule struct {
	ID       string `json:"id"`
	Record   uint32 `json:"record"`
	Once     bool   `json:"once"`
	Expr     string `json:"expr,omitempty"`
	At       int64  `json:"at,omitempty"`
	Target   string `json:"target"`
	Body     string `json:"body"`
	Creator  string `json:"creator"`
	Enabled  bool   `json:"enabled"`
	Disabled string `json:"disabled,omitempty"` // why it is not enabled: hand, failure, ended
	Until    int64  `json:"until,omitempty"`
	MaxRuns  int    `json:"max_runs,omitempty"`
	MaxLate  int64  `json:"max_late,omitempty"` // seconds; zero is no limit
	Next     int64  `json:"next,omitempty"`     // next slot to fire; zero is none
	Last     int64  `json:"last,omitempty"`     // the last slot claimed
	InFlight int64  `json:"in_flight,omitempty"` // a claimed slot whose outcome is not recorded
	Missed   int    `json:"missed,omitempty"`   // slots passed over by the in-flight firing's catch-up
	Runs     int    `json:"runs"`
	Sent     int    `json:"sent"`
	Dropped  int    `json:"dropped"`
	Streak   int    `json:"streak,omitempty"` // refused firings in a row
	Created  int64  `json:"created"`
	Changed  int64  `json:"changed"`
}

// ScheduleOutcome is one firing's result.
type ScheduleOutcome struct {
	Schedule string `json:"schedule"`
	Record   uint32 `json:"record"`
	At       int64  `json:"at"`
	Slot     int64  `json:"slot"`
	Outcome  string `json:"outcome"`
	Detail   string `json:"detail,omitempty"`
}

// ScheduleCount is how many schedules one record holds.
type ScheduleCount struct{ Recurring, Once int }

// ScheduleChange is what an edit decides: write Value, delete the schedule,
// and record Outcome, in the same transaction.
type ScheduleChange struct {
	Write, Delete bool
	Value         Schedule
	Outcome       *ScheduleOutcome
}

// Outcome retention: at most this many per record, none older than this.
const (
	ScheduleOutcomesKept = 1000
	ScheduleOutcomeAge   = 7 * 24 * 3600
)

// ScheduleStore keeps the schedules. Every edit is one transaction.
type ScheduleStore interface {
	// ScheduleGet reads one schedule by id.
	ScheduleGet(id string) (Schedule, bool, error)
	// ScheduleEdit reads schedule id and the counts of record, hands them to
	// edit and applies what it returns. It writes nothing for a record the
	// store does not hold (ErrKVNoRecord); an error from edit changes nothing.
	ScheduleEdit(id string, record uint32, edit func(old Schedule, found bool, held ScheduleCount) (ScheduleChange, error)) error
	// ScheduleList is every schedule record holds, by id.
	ScheduleList(record uint32) ([]Schedule, error)
	// ScheduleDue is up to limit enabled schedules whose next slot is at or
	// before now, earliest first.
	ScheduleDue(now int64, limit int) ([]Schedule, error)
	// ScheduleNext is the earliest next slot of any enabled schedule.
	ScheduleNext() (int64, bool, error)
	// ScheduleInFlight is every schedule with a claimed, unrecorded firing.
	ScheduleInFlight() ([]Schedule, error)
	// ScheduleCounts is how many schedules each record holding any has.
	ScheduleCounts() (map[uint32]ScheduleCount, error)
	// ScheduleOutcomes is record's kept outcomes, newest first; schedule
	// narrows them to one schedule when not empty.
	ScheduleOutcomes(record uint32, schedule string) ([]ScheduleOutcome, error)
	// ScheduleOrphans lists record IDs holding schedules that name no stored record.
	ScheduleOrphans() ([]uint32, error)
	// ScheduleBroken is the id of every stored schedule that does not
	// decode; every other call passes over it, so it never fires.
	ScheduleBroken() ([]string, error)
}
