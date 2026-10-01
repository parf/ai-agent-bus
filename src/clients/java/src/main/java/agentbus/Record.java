package agentbus;

import agentbus.BusException.TimeoutException;

import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.CancellationException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.locks.ReentrantLock;
import java.util.function.Function;

/**
 * A record the caller acts ON: another's, or its own through {@link Bus#self()}.
 * Its store, locks and private values live here, so reading somebody else's
 * config shows in the code.
 *
 * <p>Config and secret are read lazily, blocking, and cached on this object as
 * bytes. Each value has its own lock, held only while fetching: concurrent first
 * readers share one fetch, later ones read the cache without waiting, and a
 * failure reaches every waiter and is not cached. {@link #refresh()} bumps a
 * generation, so a fetch that began before it does not refill the cache.
 */
public final class Record {
    private final Http t;
    private final String name;
    private final KV kv;
    private final Locks locks;
    private final Cached config;
    private final Cached secret;
    /** Guarded by {@code this}: a refresh and a fetch's publication never interleave. */
    private long generation;

    Record(Http t, String name, Locks locks) {
        this.t = t;
        this.name = name;
        this.kv = new KV(t, name);
        this.locks = locks;
        this.config = new Cached("/config");
        this.secret = new Cached("/secret");
    }

    public String name() { return name; }

    public KV kv() { return kv; }

    public Locks locks() { return locks; }

    /** The record as the caller may see it, a parsed JSON object. */
    @SuppressWarnings("unchecked")
    public Map<String, Object> lookup() {
        return (Map<String, Object>) t.json("GET", "/lookup", Map.of("name", name), null, false);
    }

    /**
     * Changes settings as the daemon's /manage takes them: descr, addr, protocol,
     * allow, subs, maintainers, status, personal, owner, ttl, bound, overflow,
     * roles, or add / add_to_set / remove deltas.
     */
    @SuppressWarnings("unchecked")
    public Map<String, Object> manage(Map<String, Object> changes) {
        Map<String, Object> b = new LinkedHashMap<>(changes);
        b.put("name", name);
        return (Map<String, Object>) t.json("POST", "/manage", null, b, true);
    }

    public void unregister() { t.json("POST", "/unregister", null, Map.of("name", name), true); }

    /** The stored configuration as JSON text; empty when none is stored. */
    public Optional<String> config() {
        String s = new String(config.get(), java.nio.charset.StandardCharsets.UTF_8).trim();
        return s.equals("null") ? Optional.empty() : Optional.of(s);
    }

    /** The configuration parsed afresh on every call, so no caller shares a mutable value. */
    public Optional<Object> configTree() { return config().map(Json::parse); }

    /** The configuration bound by the caller's own codec, a fresh value on every call. */
    public <T> Optional<T> config(Function<String, T> bind) { return config().map(bind); }

    /** A new Secret holding its own copy of the cached bytes. An absent secret is a 404 like an unseen record. */
    public Secret secret() { return new Secret(secret.get()); }

    /**
     * Forgets both. A cached value outlives a rotated secret or a removed
     * Maintainer until this: it is a copy, and revocation never reaches it.
     */
    public synchronized void refresh() {
        generation++;
        config.drop();
        secret.drop();
    }

    private synchronized long generation() { return generation; }

    /** Publishes a fetch's result only if no refresh came since it began. */
    private synchronized void publish(Cached c, long gen, byte[] got) {
        if (gen == generation) c.value = got;
    }

    private final class Cached {
        private final String path;
        private final ReentrantLock lock = new ReentrantLock();
        private volatile byte[] value;

        Cached(String path) { this.path = path; }

        byte[] get() {
            t.open(); // a closed client answers nothing more, cached or not
            byte[] v = value;
            if (v != null) return v;
            Duration limit = t.timeout();
            try {
                if (!lock.tryLock(limit.toMillis(), TimeUnit.MILLISECONDS)) {
                    throw new TimeoutException("waited " + limit + " for another thread's read of " + path + " " + name, null);
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new CancellationException("interrupted waiting to read " + path);
            }
            try {
                if (value != null) return value;
                long gen = generation();
                byte[] got = t.request("GET", path, Map.of("name", name), null, false, null, Map.of()).body();
                // A configuration is JSON; one that is not is refused here, and never cached.
                if (path.equals("/config")) Json.parse(new String(got, java.nio.charset.StandardCharsets.UTF_8).trim());
                publish(this, gen, got);
                return got;
            } finally {
                lock.unlock();
            }
        }

        void drop() { value = null; }
    }
}
