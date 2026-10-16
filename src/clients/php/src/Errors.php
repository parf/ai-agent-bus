<?php
// The errors a call can end in. A daemon refusal is an HTTP status and
// {"error": text}; one status covers several reasons, so the subclasses go
// by status and never by parsing the text (Plans/R1.0-Release/done/client-libraries/php.md#errors).
declare(strict_types=1);

namespace AgentBus;

class BusException extends \RuntimeException {}

/** The daemon answered and turned the call away. */
class Refusal extends BusException
{
    public function __construct(private int $status, string $message)
    {
        parent::__construct($message !== '' ? $message : "refused with $status", $status);
    }

    public function status(): int { return $this->status; }

    /** The subclass for a status; any other status is a plain Refusal. */
    public static function of(int $status, string $message): self
    {
        return match ($status) {
            400 => new Invalid($status, $message),
            401 => new Unauthenticated($status, $message),
            403 => new Forbidden($status, $message),
            404 => new NotFound($status, $message),
            409 => new Busy($status, $message),
            412 => new Exists($status, $message),
            413 => new TooLarge($status, $message),
            429 => new Full($status, $message),
            default => new self($status, $message),
        };
    }
}

final class Invalid extends Refusal {}
final class Unauthenticated extends Refusal {}
final class Forbidden extends Refusal {}
/** Unknown, unseen, no secret, no such value, lock not held: told apart only by the text. */
final class NotFound extends Refusal {}
/** A lock somebody holds, or the caller's own hold taken again. */
final class Busy extends Refusal {}
final class Exists extends Refusal {}
final class TooLarge extends Refusal {}
/** The recipient's inbox is full. */
final class Full extends Refusal {}

/** Nothing was sent: no connection, or TLS failed before the request. */
final class Transport extends BusException {}

/** The client's own deadline ran out on a read. */
final class Timeout extends BusException {}

/** A write left and its answer never came: it may have happened. Never retried. */
final class UncertainOutcome extends BusException {}

/** The credential names somebody other than the name the client was told to expect. */
final class IdentityMismatch extends BusException {}

/** Caught here, before any request. */
final class InvalidArgument extends BusException {}

/** A lease used after its own known expiry. */
final class LeaseExpired extends BusException {}
