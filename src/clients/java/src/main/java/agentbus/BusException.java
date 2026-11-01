package agentbus;

/**
 * Everything a call can end in, unchecked. A daemon refusal is an HTTP status
 * and {@code {"error": text}}; one status covers several reasons, so the
 * subclasses go by status and never by parsing the text
 * (Plans/R1.0-Release/done/client-libraries/java.md#errors).
 */
public class BusException extends RuntimeException {
    public BusException(String message) { super(message); }

    public BusException(String message, Throwable cause) { super(message, cause); }

    /** The daemon answered and turned the call away. */
    public static class RefusalException extends BusException {
        private final int status;
        private final String kind;

        public RefusalException(int status, String message) { this(status, message, null); }

        public RefusalException(int status, String message, String kind) {
            super(message.isEmpty() ? "refused with " + status : message);
            this.status = status;
            this.kind = kind;
        }

        public int status() { return status; }

        /**
         * The refusal's {@code kind} as the daemon sent it, or null when it sent none: acl, authority,
         * busy, credential, enrolment, full, malformed, name-taken, second-reader, suspended,
         * unavailable, not_found or no_route.
         */
        public String kind() { return kind; }

        static RefusalException of(int status, String message, String kind) {
            return switch (status) {
                case 400 -> new Invalid(status, message, kind);
                case 401 -> new Unauthenticated(status, message, kind);
                case 403 -> new Forbidden(status, message, kind);
                case 404 -> new NotFound(status, message, kind);
                case 409 -> new Busy(status, message, kind);
                case 412 -> new Exists(status, message, kind);
                case 413 -> new TooLarge(status, message, kind);
                case 429 -> new Full(status, message, kind);
                default -> new RefusalException(status, message, kind);
            };
        }
    }

    public static final class Invalid extends RefusalException { Invalid(int s, String m) { super(s, m); }
        Invalid(int s, String m, String k) { super(s, m, k); } }
    public static final class Unauthenticated extends RefusalException { Unauthenticated(int s, String m) { super(s, m); }
        Unauthenticated(int s, String m, String k) { super(s, m, k); } }
    public static final class Forbidden extends RefusalException { Forbidden(int s, String m) { super(s, m); }
        Forbidden(int s, String m, String k) { super(s, m, k); } }
    /** Unknown, unseen, no secret, no such value, lock not held: told apart only by the text. */
    public static final class NotFound extends RefusalException { NotFound(int s, String m) { super(s, m); }
        NotFound(int s, String m, String k) { super(s, m, k); } }
    /** A lock somebody holds, or the caller's own hold taken again. */
    public static final class Busy extends RefusalException { Busy(int s, String m) { super(s, m); }
        Busy(int s, String m, String k) { super(s, m, k); } }
    public static final class Exists extends RefusalException { Exists(int s, String m) { super(s, m); }
        Exists(int s, String m, String k) { super(s, m, k); } }
    public static final class TooLarge extends RefusalException { TooLarge(int s, String m) { super(s, m); }
        TooLarge(int s, String m, String k) { super(s, m, k); } }
    /** The recipient's inbox is full. */
    public static final class Full extends RefusalException { Full(int s, String m) { super(s, m); }
        Full(int s, String m, String k) { super(s, m, k); } }

    /** Nothing was sent: no connection, or TLS failed before the request. */
    public static final class TransportException extends BusException {
        public TransportException(String m, Throwable c) { super(m, c); }
    }

    /** The client's own deadline ran out on a read. */
    public static final class TimeoutException extends BusException {
        public TimeoutException(String m, Throwable c) { super(m, c); }
    }

    /** A write left and its answer never came: it may have happened. Never retried. */
    public static final class UncertainOutcomeException extends BusException {
        public UncertainOutcomeException(String m, Throwable c) { super(m, c); }
    }

    /** The credential names somebody other than the name the client was told to expect. */
    public static final class IdentityMismatchException extends BusException {
        public IdentityMismatchException(String m) { super(m); }
    }

    /** The daemon refuses clients as old as this one: its {@code protocol_min} is above {@link Bus#CLIENT_PROTOCOL}. */
    public static final class DaemonTooNewException extends BusException {
        public DaemonTooNewException(String m) { super(m); }
    }

    /** The daemon's {@code /identity} answer broke the protocol rule; the message names the member. */
    public static final class ProtocolErrorException extends BusException {
        public ProtocolErrorException(String m) { super(m); }
    }

    /** A lease used after its own known expiry, or after it was closed. */
    public static final class LeaseExpiredException extends BusException {
        public LeaseExpiredException(String m) { super(m); }
    }
}
