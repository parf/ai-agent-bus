# TODO R1.2

## Objective

Explore the [later topics](README.md#scope) after the catalogue provides evidence. Unscheduled; no implementation commitment.

## Next step

For each [open question](QUESTIONS.md#open-questions), compare an ordinary service with a daemon capability using a concrete caller and failure case. Ask the owner to choose before defining storage, identity or protocol structures.

## Removed names protection

Restoring any protection for a [removed name](README.md#removed-names) starts
with the owner answering who it belongs to and for how long; MVP's answer was
"its last owner, forever", which is what was removed. Unscheduled.

Falsifiable acceptance, when it is scheduled: a removed name asked for by a
stranger is refused, and the check fails if the refusal is silence or if the
name's own holder cannot take it back. Removing the reservation must break a
named check, not merely leave one passing.

## Dependencies

The R1.1 catalogue must establish which services need shared state and which daemon components can actually work behind the public interface. A proposal is ready for a release plan when its owner, scope, dependencies and falsifiable acceptance are explicit.
