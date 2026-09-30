package locks

import (
	"errors"
	"fmt"

)

// ErrNotHeld: nobody holds what a release came for.
var ErrNotHeld = errors.New("nobody holds that lock")

// HeldBy names the holder of a lock that is not the caller's to release.
type HeldBy struct{ Holder string }

func (h *HeldBy) Error() string { return fmt.Sprintf("held by %s; --force releases another holder's lock", h.Holder) }

// Release takes the error apart for the API's refusal mapping.
func Reason(err error) (string, bool) {
	var by *HeldBy
	if errors.As(err, &by) {
		return by.Holder, true
	}
	return "", false
}

// Is makes any HeldBy match the codes table's sentinel, whatever holder it
// names: the refusal kind is busy whoever holds it.
func (h *HeldBy) Is(target error) bool {
	_, ok := target.(*HeldBy)
	return ok
}
