package locks

import (
	"errors"
	"fmt"
)

// ErrNotHeld: nobody holds what a release or an extend came for.
var ErrNotHeld = errors.New("nobody holds that lock")

// ErrRecordGone ends a pending take when its record is removed or deactivated.
var ErrRecordGone = errors.New("the lock record ended")

// HeldBy names the holder of a lock that is not the caller's to release,
// extend or take.
type HeldBy struct{ Holder string }

func (h *HeldBy) Error() string {
	return fmt.Sprintf("held by %s; --force releases another holder's lock", h.Holder)
}

// Is makes any HeldBy match the codes table's sentinel, whatever holder it
// names: the refusal kind is busy whoever holds it.
func (h *HeldBy) Is(target error) bool { _, ok := target.(*HeldBy); return ok }

// Displaced is a --force release's answer: the hold ended, and this is who
// lost it, for the audit to say.
type Displaced struct{ Previous string }

func (d *Displaced) Error() string { return fmt.Sprintf("released %s's lock", d.Previous) }
