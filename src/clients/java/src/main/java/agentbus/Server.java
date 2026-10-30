package agentbus;

import agentbus.BusException.RefusalException;
import agentbus.Options.Consume;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Semaphore;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static java.lang.System.Logger.Level.WARNING;

/**
 * Serves a Bus's inbox. One reader thread; handlers on virtual threads, at most
 * {@code workers} at once. The reader takes a permit before each unfiltered
 * consume, so nothing is taken from the daemon while every handler is busy — a
 * taken message is lost if the process dies. Handlers' own calls still get
 * their replies at capacity, through their filtered waits.
 *
 * <p>Each request is acked, then handed to the handler; its {@link Response}
 * is sent as a reply or a done. A throwing handler is reported (type only by
 * default) and sends nothing more. A request whose caller already gave up is
 * not run, and an answer or a receipt is no request. A refusal from the
 * daemon — the credential revoked, the name gone — ends the server, and
 * {@link #failure()} says why; a lost connection is waited out.
 *
 * <p>Stopping, by {@link #close()} or by such a refusal, takes no more work,
 * waits up to the drain for running handlers, then cancels and interrupts them.
 */
public final class Server implements AutoCloseable {
    /** What a request is answered with. */
    public sealed interface Response permits Response.Reply, Response.Done, Response.None {
        record Reply(String body) implements Response {}
        record Done() implements Response {}
        record None() implements Response {}
        static Response reply(String body) { return new Reply(body); }
        static Response done() { return new Done(); }
        static Response none() { return new None(); }
    }

    /** One request's context: its roles, its caller's absolute deadline as received, and whether a stop has cancelled it. */
    public static final class Context {
        private final Message message;
        private volatile boolean cancelled;

        Context(Message m) { this.message = m; }

        public Message message() { return message; }
        public List<String> roles() { return message.roles(); }
        public Optional<Instant> deadline() { return message.deadline(); }
        public boolean cancelled() { return cancelled; }
    }

    @FunctionalInterface
    public interface Handler {
        Response handle(Message m, Context ctx) throws Exception;
    }

    private static final ThreadLocal<Server> HANDLING = new ThreadLocal<>();

    private final Bus bus;
    private final Handler handler;
    private final Options.Serve o;
    private final Semaphore permits;
    private final ExecutorService pool = Executors.newVirtualThreadPerTaskExecutor();
    private final Set<Context> running = ConcurrentHashMap.newKeySet();
    private final CountDownLatch finished = new CountDownLatch(1);
    private final AtomicInteger served = new AtomicInteger();
    private final AtomicBoolean stopping = new AtomicBoolean();
    private final AtomicReference<Throwable> failure = new AtomicReference<>();
    private Thread reader;

    Server(Bus bus, Handler handler, Options.Serve o) {
        this.bus = bus;
        this.handler = handler;
        this.o = o;
        this.permits = new Semaphore(o.workers(), true);
    }

    void start() { reader = Thread.ofVirtual().name("agent-bus serve").start(this::run); }

    /** How many requests have been handed to a handler. */
    public int served() { return served.get(); }

    /** The refusal that ended the server, if one did. */
    public Optional<Throwable> failure() { return Optional.ofNullable(failure.get()); }

    private void run() {
        try {
            read();
        } finally {
            drain();
        }
    }

    private void read() {
        Consume c = new Consume(Optional.empty(), o.topic(), Optional.empty(), o.poll(), o.share());
        while (!stopping.get()) {
            try {
                if (!permits.tryAcquire(o.poll().toMillis(), TimeUnit.MILLISECONDS)) continue;
            } catch (InterruptedException e) {
                return;
            }
            Message m;
            try {
                m = bus.take(c, o.poll()).orElse(null);
            } catch (RefusalException e) {
                permits.release();
                failure.compareAndSet(null, e);
                Bus.LOG.log(WARNING, "agent-bus serve: the daemon refused the inbox read (" + e.status() + "); stopping");
                return;
            } catch (BusException.UncertainOutcomeException e) {
                // A read lost mid-answer may have taken a message: never read on as if not.
                permits.release();
                if (stopping.get()) return;
                failure.compareAndSet(null, e);
                Bus.LOG.log(WARNING, "agent-bus serve: an inbox read was lost mid-answer and may have taken a message; stopping");
                return;
            } catch (RuntimeException e) {
                permits.release();
                if (stopping.get()) return;
                Bus.LOG.log(WARNING, "agent-bus serve: " + e.getClass().getSimpleName() + " reading the inbox; trying again");
                try { Thread.sleep(500); } catch (InterruptedException ie) { return; }
                continue;
            }
            if (m == null || bus.route(m) || !request(m)) {
                permits.release();
                continue;
            }
            run(m);
        }
    }

    /**
     * Runs a self-call's own request, which the call's wait took instead of this
     * reader, as ordinary work once a handler is free. False when it cannot run:
     * stopping, not a request, or no handler free within {@code within}.
     */
    boolean work(Message m, Duration within) throws InterruptedException {
        if (stopping.get() || !request(m)) return false;
        if (!permits.tryAcquire(within.toMillis(), TimeUnit.MILLISECONDS)) return false;
        try {
            if (!stopping.get()) return run(m);
        } catch (java.util.concurrent.RejectedExecutionException e) {
            // Stopped between the check and the submit: the drain no longer takes work.
        }
        permits.release();
        return false;
    }

    // Acks a request and hands it to a handler; the caller holds its permit.
    private boolean run(Message m) {
        try {
            bus.ack(m);
        } catch (RuntimeException e) {
            Bus.LOG.log(WARNING, "agent-bus serve: could not ack " + m.id() + ": " + e.getClass().getSimpleName());
        }
        Context ctx = new Context(m);
        running.add(ctx);
        try {
            pool.submit(() -> handle(m, ctx));
        } catch (RuntimeException e) {
            running.remove(ctx);
            throw e;
        }
        served.incrementAndGet();
        return true;
    }

    /** Whether to run it: not a receipt, not an answer to somebody's call, not too late. */
    private boolean request(Message m) {
        if (m.isReceipt() || m.re().isPresent()) return false;
        if (m.tooLate(Instant.now())) {
            Bus.LOG.log(WARNING, "agent-bus serve: " + m.id() + " arrived after its caller gave up; not run");
            return false;
        }
        return true;
    }

    private void handle(Message m, Context ctx) {
        HANDLING.set(this);
        try {
            Response r = handler.handle(m, ctx);
            switch (r) {
                case Response.Reply rep -> bus.reply(m, rep.body());
                case Response.Done d -> bus.done(m);
                case Response.None n -> { }
                case null -> bus.done(m);
            }
        } catch (Throwable e) {
            if (o.onError().isPresent()) o.onError().get().accept(m, e);
            // The type only: a message may carry the request or a private value.
            else Bus.LOG.log(WARNING, "agent-bus serve: handler failed on " + m.id() + ": " + e.getClass().getName());
        } finally {
            HANDLING.remove();
            running.remove(ctx);
            permits.release();
        }
    }

    // Runs whatever interrupted the reader: the drain and the cancellation
    // after it are the point, so an interrupt does not cut them short.
    private void drain() {
        stopping.set(true);
        try {
            pool.shutdown();
            if (!awaitQuietly(o.drain())) {
                for (Context ctx : running) ctx.cancelled = true;
                pool.shutdownNow();
                awaitQuietly(Duration.ofSeconds(5));
            }
        } finally {
            bus.served(this);
            finished.countDown();
        }
    }

    private boolean awaitQuietly(Duration d) {
        long until = System.nanoTime() + d.toNanos();
        while (true) {
            try {
                return pool.awaitTermination(Math.max(0, until - System.nanoTime()), TimeUnit.NANOSECONDS);
            } catch (InterruptedException e) {
                // keep waiting: the bound is the drain, not an interrupt (which this cleared)
            }
        }
    }

    /** Waits until the server has stopped. */
    public void awaitTermination() throws InterruptedException { finished.await(); }

    /**
     * Stops taking work and waits for the drain. From inside a handler it only
     * asks: waiting there would wait on itself.
     */
    @Override
    public void close() {
        stopping.set(true);
        if (HANDLING.get() == this) return;
        if (reader != null) reader.interrupt();
        try {
            finished.await();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }
}
