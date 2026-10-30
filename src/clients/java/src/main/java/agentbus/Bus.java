package agentbus;

import agentbus.BusException.IdentityMismatchException;
import agentbus.Message.ReplyTo;
import agentbus.Options.Call;
import agentbus.Options.Connect;
import agentbus.Options.Consume;
import agentbus.Options.Send;

import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.CancellationException;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.Semaphore;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

/**
 * The caller: whoever the credential says (docs/02-access.md#what-a-call-carries).
 * One Bus is thread-safe and meant to be shared. A name never makes it somebody
 * else; {@link Connect#expectName} only checks it.
 *
 * <p>Replies reach a {@link #call} through its own filtered wait on topic and
 * tag, which the daemon serves ahead of any unfiltered reader
 * (docs/04-messaging.md#one-reader-per-inbox). An unfiltered read — {@link #serve}
 * or {@link #consume} — that takes a pending call's message anyway hands it to
 * that call. Plans/R1.0-Release/done/client-libraries/java.md owns the design.
 */
public final class Bus implements AutoCloseable {
    private static final SecureRandom RANDOM = new SecureRandom();
    static final System.Logger LOG = System.getLogger("agentbus");

    final Http t;
    private final Optional<String> expect;
    private volatile String name;
    private final ConcurrentHashMap<String, Record> records = new ConcurrentHashMap<>();
    private final ConcurrentHashMap<String, Semaphore> gates = new ConcurrentHashMap<>();
    private final ConcurrentHashMap<String, Pending> pending = new ConcurrentHashMap<>();
    private final AtomicReference<Server> serving = new AtomicReference<>();
    /** Test only: false leaves a call's replies to whatever unfiltered reader runs. */
    volatile boolean pollReplies = true;

    private Bus(Http t, Optional<String> expect) {
        this.t = t;
        this.expect = expect.map(s -> s.strip().toLowerCase());
    }

    /** A client. With an expected name, this asks the daemon who the credential is and refuses somebody else. */
    public static Bus connect(Connect o) {
        Bus b = new Bus(new Http(o.addr(), o.token(), o.timeout(), o.pin().orElse(null)), o.expectName());
        if (o.expectName().isPresent()) b.name();
        return b;
    }

    /** Who the credential is, as the daemon answers it. */
    public String name() { return who(t.timeout()); }

    private String who(Duration limit) {
        String n = name;
        if (n == null) {
            n = String.valueOf(((Map<?, ?>) t.json("GET", "/status", null, null, false, limit)).get("you"));
            if (expect.isPresent() && !expect.get().equals(n)) {
                throw new IdentityMismatchException("the credential is " + n + ", not " + expect.get());
            }
            name = n;
        }
        return n;
    }

    /** Totals, counters, and who the caller is with what authority. */
    @SuppressWarnings("unchecked")
    public Map<String, Object> status() { return (Map<String, Object>) t.json("GET", "/status", null, null, false); }

    /** The node: its release, owner and host. Needs no credential. */
    @SuppressWarnings("unchecked")
    public Map<String, Object> identity() { return (Map<String, Object>) t.json("GET", "/identity", null, null, false); }

    /** The caller's own record. */
    public Record self() { return record(name()); }

    /** A record to act on. The same object for the same name, so its cached values are shared. */
    public Record record(String name) {
        return records.computeIfAbsent(name, n -> new Record(t, n, new Locks(t, n, gates)));
    }

    /** The records the caller may see, optionally of one kind. */
    @SuppressWarnings("unchecked")
    public List<Map<String, Object>> list(Optional<String> kind) {
        return (List<Map<String, Object>>) t.json("GET", "/ls", kind.map(k -> Map.of("kind", k)).orElse(Map.of()), null, false);
    }

    /** States a record, owned by the caller. */
    public Record register(Map<String, Object> spec) {
        Object r = t.json("POST", "/register", null, spec, true);
        return record(String.valueOf(((Map<?, ?>) r).get("name")));
    }

    /** Accepted by the bus; that says nothing about anybody reading it. Answers the message id. */
    public String send(String to, String body, Send o) { return post(to, body, o, null, null); }

    public String send(String to, String body) { return send(to, body, Send.none()); }

    /** A publication: a send to a channel, which copies it to its Deliver-To list. */
    public String publish(String channel, String body, Send o) { return send(channel, body, o); }

    /** Takes the caller itself off a channel's Deliver-To list. */
    public void unsubscribe(String channel) {
        t.json("POST", "/subscribe", null, Map.of("channel", channel, "off", true), true);
    }

    /**
     * The next message within the wait, or empty. Refused while {@link #serve}
     * runs, which owns the unfiltered read. A message a pending call is waiting
     * for goes to that call, and the wait goes on.
     */
    public Optional<Message> consume(Consume o) {
        if (serving.get() != null) throw new IllegalStateException("consume() while serve() runs would take its messages");
        Instant until = Instant.now().plus(o.within());
        while (true) {
            Duration left = Duration.between(Instant.now(), until);
            Optional<Message> m = take(o, left.isNegative() ? Duration.ZERO : left);
            if (m.isEmpty() || !route(m.get())) return m;
            if (!Instant.now().isBefore(until)) return Optional.empty();
        }
    }

    /**
     * Sends and waits for the answer, matched on a fresh tag. An ack is noted; a
     * done or an answer ends it; the deadline ends it as TimedOut. An interrupt
     * throws {@link CancellationException}. Neither stops the remote work.
     */
    public Outcome call(String to, String body, Call o) {
        if (o.timeout().isNegative() || o.timeout().isZero()) throw new IllegalArgumentException("a call needs a positive timeout");
        // One budget for the whole exchange, from before anything is sent: the
        // identity check, the send and every wait each get what is left of it.
        Instant until = Instant.now().plus(o.timeout());
        t.open();
        String tag = "call-" + HexFormat.of().formatHex(bytes(8));
        String inbox = o.replyTo().orElse(null);
        Pending p = new Pending(inbox == null ? who(left(until)) : inbox);
        String key = key(o.topic(), tag);
        pending.put(key, p);
        try {
            Send s = Send.topic(o.topic()).tag(tag).waitFor(o.timeout());
            if (o.ttl().isPresent()) s = s.ttl(o.ttl().get());
            if (inbox != null) s = s.replyTo(ReplyTo.of(inbox, o.topic(), tag));
            if (o.ask().isPresent()) s = s.ask(o.ask().get());
            p.id = post(to, body, s, null, null, left(until));
            // Tests switch the call's own wait off to prove an unfiltered reader routes a stray reply.
            Thread poller = Thread.ofVirtual().name("agent-bus call " + tag)
                    .start(pollReplies ? () -> poll(p, o.topic(), tag, inbox, until) : () -> { });
            try {
                boolean acked = false;
                while (true) {
                    long left = Duration.between(Instant.now(), until).toMillis();
                    if (left <= 0) return new Outcome.TimedOut(acked);
                    Message m = p.queue.poll(left, TimeUnit.MILLISECONDS);
                    if (m == null) return new Outcome.TimedOut(acked);
                    if (m == POISON) throw p.failure.get();
                    // A receipt is this call's only when it names this call's message.
                    if (m.isReceipt() && !m.re().orElse("").equals(p.id)) continue;
                    if (m.receipt().orElse(null) == Message.Receipt.ACK) { acked = true; continue; }
                    if (m.receipt().orElse(null) == Message.Receipt.DONE) return new Outcome.Done(m);
                    return new Outcome.Answer(m, acked);
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new CancellationException("call to " + to + " interrupted");
            } finally {
                p.over = true;
                poller.interrupt();
            }
        } finally {
            pending.remove(key);
        }
    }

    /** Answers a message where it asked: its reply_to, or its sender, with its topic and tag. */
    public String reply(Message m, String body) { return respond(m, body, null); }

    public void ack(Message m) { respond(m, "", "ack"); }

    public void done(Message m) { respond(m, "", "done"); }

    /**
     * Serves the caller's inbox with bounded handlers until the returned Server
     * is closed. See {@link Server}.
     */
    public Server serve(Server.Handler handler, Options.Serve o) {
        Server s = new Server(this, handler, o);
        if (!serving.compareAndSet(null, s)) throw new IllegalStateException("this Bus is already serving");
        s.start();
        return s;
    }

    void served(Server s) { serving.compareAndSet(s, null); }

    /**
     * Closes any server first, then the client: every later call through it, or
     * through a Record it handed out, cached values included, is refused.
     */
    @Override
    public void close() {
        Server s = serving.get();
        if (s != null) s.close();
        t.close();
        // Calls still waiting hear it now, not at their deadlines.
        for (Pending p : pending.values()) {
            p.failure.compareAndSet(null, new BusException("this client was closed"));
            p.queue.offer(POISON);
        }
    }

    // The filtered wait of one call, until it is over or its budget is: each
    // request bounded by what is left. A failure before the budget is the
    // call's to throw, not a reason to wait on in silence.
    private void poll(Pending p, String topic, String tag, String inbox, Instant until) {
        Consume c = new Consume(Optional.ofNullable(inbox), Optional.of(topic), Optional.of(tag), Duration.ZERO, false);
        while (!p.over) {
            Duration left = left(until);
            if (!Instant.now().isBefore(until)) return;
            try {
                Duration wait = left.compareTo(Duration.ofSeconds(30)) > 0 ? Duration.ofSeconds(30) : left;
                Optional<Message> m = take(c, wait, left);
                m.ifPresent(p.queue::offer);
            } catch (CancellationException e) {
                return;
            } catch (RuntimeException e) {
                // The budget running out inside the wait is the call's timeout.
                if (p.over || !Instant.now().isBefore(until.minusMillis(5))) return;
                p.failure.compareAndSet(null, e);
                p.queue.offer(POISON);
                return;
            }
        }
    }

    private static Duration left(Instant until) {
        Duration d = Duration.between(Instant.now(), until);
        return d.isNegative() || d.isZero() ? Duration.ofMillis(1) : d;
    }

    /** Hands a message to the pending call it belongs to, if any. */
    boolean route(Message m) {
        Pending p = pending.get(key(m.topic().orElse(""), m.tag().orElse("")));
        if (p == null || p.over) return false;
        return p.queue.offer(m);
    }

    Optional<Message> take(Consume o, Duration wait) { return take(o, wait, t.timeout().plus(wait)); }

    /**
     * One consume, its whole request bounded by {@code limit}. Consuming takes the
     * message off the queue, so a failure once the request has left may have
     * lost one: uncertain, as a send is, and never retried.
     */
    Optional<Message> take(Consume o, Duration wait, Duration limit) {
        Map<String, String> q = new LinkedHashMap<>();
        q.put("wait", wait.toMillis() + "ms");
        o.inbox().ifPresent(v -> q.put("inbox", v));
        o.topic().ifPresent(v -> q.put("topic", v));
        o.tag().ifPresent(v -> q.put("tag", v));
        if (o.share()) q.put("share", "1");
        Http.Response r = t.within("GET", "/consume", q, null, true, limit, Map.of());
        if (r.status() == 204 || r.body().length == 0) return Optional.empty();
        return Optional.of(Message.of(Json.object(r.text())));
    }

    private String respond(Message m, String body, String receipt) {
        ReplyTo r = m.route();
        Send s = Send.none();
        if (r.topic().isPresent()) s = s.withTopic(r.topic().get());
        if (r.tag().isPresent()) s = s.tag(r.tag().get());
        return post(r.name(), body, s, receipt, m.id());
    }

    private String post(String to, String body, Send o, String receipt, String re) {
        return post(to, body, o, receipt, re, t.timeout());
    }

    private String post(String to, String body, Send o, String receipt, String re, Duration limit) {
        Map<String, Object> e = new LinkedHashMap<>();
        e.put("to", to);
        e.put("body", body);
        o.topic().ifPresent(v -> e.put("topic", v));
        o.tag().ifPresent(v -> e.put("tag", v));
        o.ttl().ifPresent(v -> e.put("ttl", v.toMillis() + "ms"));
        // A wait of nothing is no wait: the daemon refuses a zero one.
        o.waitFor().filter(v -> !v.isZero()).ifPresent(v -> e.put("wait", v.toMillis() + "ms"));
        o.replyTo().ifPresent(v -> e.put("reply_to", v.toJson()));
        o.ask().ifPresent(v -> e.put("ask", v));
        if (receipt != null) e.put("receipt", receipt);
        if (re != null) e.put("re", re);
        Object a = t.json("POST", "/send", null, e, true, limit);
        return String.valueOf(((Map<?, ?>) a).get("message_id"));
    }

    private static String key(String topic, String tag) { return topic + "\u0000" + tag; }

    private static byte[] bytes(int n) {
        byte[] b = new byte[n];
        RANDOM.nextBytes(b);
        return b;
    }

    /** The CLI's discovery: the account's own socket without a token, the shared one with. */
    static String discover(boolean hasToken) {
        String xdg = System.getenv("XDG_RUNTIME_DIR");
        String account = System.getProperty("user.name", "");
        for (String dir : xdg != null && !xdg.isEmpty() ? List.of(xdg + "/agent-bus", "/run/agent-bus") : List.of("/run/agent-bus")) {
            Path p = Path.of(dir, !hasToken && !account.isEmpty() ? "user-" + account + ".sock" : "bus.sock");
            if (Files.exists(p) && !Files.isRegularFile(p) && !Files.isDirectory(p)) return p.toString();
        }
        if (xdg != null && !xdg.isEmpty()) return xdg + "/agent-bus/bus.sock";
        return System.getProperty("java.io.tmpdir") + File.separator + "agent-bus-" + uid() + ".sock";
    }

    private static String uid() {
        try {
            return String.valueOf(Files.getAttribute(Path.of(System.getProperty("user.home")), "unix:uid"));
        } catch (Exception e) {
            return "0";
        }
    }

    /** Put in a call's queue when its wait failed; the failure is the call's. */
    private static final Message POISON = new Message("", "", "", Optional.empty(), Optional.empty(), "", Instant.EPOCH,
            Optional.empty(), Optional.empty(), Optional.empty(), Optional.empty(), List.of(), Optional.empty(), 0, Optional.empty(), Optional.empty());

    private static final class Pending {
        final String inbox;
        final AtomicReference<RuntimeException> failure = new AtomicReference<>();
        final BlockingQueue<Message> queue = new LinkedBlockingQueue<>();
        volatile String id;
        volatile boolean over;

        Pending(String inbox) { this.inbox = inbox; }
    }
}
