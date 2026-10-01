package agentbus;

import agentbus.BusException.Busy;
import agentbus.BusException.LeaseExpiredException;

import java.time.Duration;
import java.time.Instant;
import java.time.OffsetDateTime;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.CancellationException;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Semaphore;
import java.util.concurrent.TimeUnit;

/**
 * A record's shared locks (docs/01-identity-and-authority.md#shared-locks).
 * A hold belongs to the caller's NAME, not a thread: the daemon refuses a
 * second take by the same name at once rather than queueing it. So acquires
 * by threads of one {@link Bus} queue first behind an in-process fair gate per
 * (record, lock), and only then ask the daemon.
 */
public final class Locks {
    /** A hold as the daemon lists it. */
    public record Hold(String holder, Instant expires) {}

    private final Http t;
    private final String record;
    private final ConcurrentHashMap<String, Semaphore> gates;

    Locks(Http t, String record, ConcurrentHashMap<String, Semaphore> gates) {
        this.t = t;
        this.record = record;
        this.gates = gates;
    }

    private Semaphore gate(String name) { return gates.computeIfAbsent(record + "\u0000" + name, k -> new Semaphore(1, true)); }

    /**
     * Takes the lock, waiting up to {@code wait} — first for another thread of
     * this Bus, then at the daemon (which caps a wait at 60 s). A refusal throws
     * {@link Busy}. A wait cancelled by interrupt may still have been granted;
     * the hold then ends by its ttl.
     */
    public Lease acquire(String name, Duration ttl, Duration wait) {
        Instant until = Instant.now().plus(wait);
        Semaphore g = gate(name);
        try {
            if (!g.tryAcquire(Math.max(0, wait.toMillis()), TimeUnit.MILLISECONDS)) {
                throw new Busy(409, "lock " + name + " on " + record + " is held by another thread of this client");
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new CancellationException("interrupted waiting for lock " + name);
        }
        try {
            Duration left = Duration.between(Instant.now(), until);
            if (left.isNegative()) left = Duration.ZERO;
            Instant start = Instant.now();
            t.request("POST", "/lock", Map.of("wait", left.toMillis() + "ms"), body(name, ttl), true, left, Map.of());
            return new Lease(this, name, start.plus(ttl), g);
        } catch (RuntimeException e) {
            g.release();
            throw e;
        }
    }

    /** acquire with no wait. A refusal propagates as {@link Busy}: held by somebody and held by this name are both 409. */
    public Lease tryAcquire(String name, Duration ttl) {
        Semaphore g = gate(name);
        if (!g.tryAcquire()) throw new Busy(409, "lock " + name + " on " + record + " is held by another thread of this client");
        try {
            Instant start = Instant.now();
            t.json("POST", "/try-lock", null, body(name, ttl), true);
            return new Lease(this, name, start.plus(ttl), g);
        } catch (RuntimeException e) {
            g.release();
            throw e;
        }
    }

    void extend(String name, Duration ttl) { t.json("POST", "/extend", null, body(name, ttl), true); }

    void release(String name) { t.json("POST", "/release", null, Map.of("record", record, "name", name), true); }

    /** Releases whoever holds it; audited. Answers the displaced holder. */
    public Optional<String> forceRelease(String name) {
        Object a = t.json("POST", "/release-force", null, Map.of("record", record, "name", name), true);
        return a instanceof Map<?, ?> m && m.get("holder") instanceof String h && !h.isEmpty() ? Optional.of(h) : Optional.empty();
    }

    public Map<String, Hold> holders() {
        Object a = t.json("GET", "/holders", Map.of("record", record), null, false);
        Map<String, Hold> out = new LinkedHashMap<>();
        if (a instanceof Map<?, ?> m && m.get("locks") instanceof Map<?, ?> ls) {
            ls.forEach((k, v) -> {
                Map<?, ?> h = (Map<?, ?>) v;
                out.put(String.valueOf(k), new Hold(String.valueOf(h.get("holder")), OffsetDateTime.parse(String.valueOf(h.get("expires"))).toInstant()));
            });
        }
        return out;
    }

    private Map<String, Object> body(String name, Duration ttl) {
        if (ttl.isNegative() || ttl.isZero()) throw new IllegalArgumentException("a ttl is positive");
        Map<String, Object> b = new LinkedHashMap<>();
        b.put("record", record);
        b.put("name", name);
        b.put("ttl", ttl.toMillis() + "ms");
        return b;
    }

    /**
     * A hold as this client took it. A convenience, not a fence: a release names
     * only the record, the lock and the caller, so a stale Lease can release a
     * later hold by the same name. It refuses locally once past its own known
     * expiry — counted from before the request left, so never later than the
     * daemon's — which narrows that window and cannot close it.
     */
    public static final class Lease implements AutoCloseable {
        private final Locks locks;
        private final String name;
        private final Semaphore gate;
        private volatile Instant expires;
        private boolean closed;

        Lease(Locks locks, String name, Instant expires, Semaphore gate) {
            this.locks = locks;
            this.name = name;
            this.expires = expires;
            this.gate = gate;
        }

        public String name() { return name; }

        /** When the hold ends unless extended, as far as this client knows. */
        public Instant expires() { return expires; }

        public synchronized void extend(Duration ttl) {
            if (closed) throw new LeaseExpiredException("lock " + name + " was released");
            if (!Instant.now().isBefore(expires)) throw new LeaseExpiredException("lock " + name + " is past its ttl");
            Instant start = Instant.now();
            locks.extend(name, ttl);
            expires = start.plus(ttl);
        }

        /** Releases the hold; nothing is sent once past its known expiry. Answers whether it sent. */
        public synchronized boolean release() {
            if (closed) return false;
            closed = true;
            try {
                if (!Instant.now().isBefore(expires)) return false;
                locks.release(name);
                return true;
            } finally {
                gate.release();
            }
        }

        /**
         * {@link #release()} for try-with-resources, quiet when the hold is
         * already gone — expired or force-released — since that is what closing
         * wants. An explicit release() reports it.
         */
        @Override
        public void close() {
            try {
                release();
            } catch (BusException.NotFound gone) {
                // not held any more: nothing left to let go of
            }
        }
    }
}
