package agentbus;

import agentbus.Message.ReplyTo;

import java.time.Duration;
import java.util.Optional;

/** The options each call takes: existing envelope and query fields, nothing new on the wire. */
public final class Options {
    private Options() {}

    /** How a {@link Bus} reaches the daemon and who it expects to be. */
    public record Connect(String addr, String token, Optional<String> expectName, Duration timeout, Optional<String> pin) {
        /** The CLI's defaults: AGENT_BUS_ADDR, AGENT_BUS_TOKEN, AGENT_BUS_NAME and AGENT_BUS_TLS_FINGERPRINT. */
        public static Connect env() {
            String token = System.getenv().getOrDefault("AGENT_BUS_TOKEN", "");
            String addr = System.getenv().get("AGENT_BUS_ADDR");
            return new Connect(addr != null && !addr.isEmpty() ? addr : Bus.discover(!token.isEmpty()), token,
                    Optional.ofNullable(System.getenv("AGENT_BUS_NAME")).filter(s -> !s.isEmpty()), Duration.ofSeconds(30),
                    Optional.ofNullable(System.getenv("AGENT_BUS_TLS_FINGERPRINT")).filter(s -> !s.isBlank()));
        }

        public static Connect of(String addr, String token) {
            return new Connect(addr, token, Optional.empty(), Duration.ofSeconds(30), Optional.empty());
        }

        public Connect expectName(String name) { return new Connect(addr, token, Optional.of(name), timeout, pin); }

        public Connect timeout(Duration d) { return new Connect(addr, token, expectName, d, pin); }

        /** An https:// daemon's certificate SHA-256 ({@code sha256:hex}, colons and case ignored): the only trust. */
        public Connect pin(String fingerprint) { return new Connect(addr, token, expectName, timeout, Optional.of(fingerprint)); }

        /** A record would print every component; the bearer token is never one of them. */
        @Override
        public String toString() {
            return "Connect[addr=" + addr + ", token=[redacted], expectName=" + expectName + ", timeout=" + timeout + ", pin=" + pin + "]";
        }
    }

    /**
     * A send: topic, tag, ttl (how long it is worth delivering), wait (how long the sender waits), reply_to,
     * and ask ({@code help}, {@code help-config}, {@code help-secret}, {@code help-json} or {@code mcp}: asks the receiver about itself instead of giving it work; the body must be empty).
     */
    public record Send(Optional<String> topic, Optional<String> tag, Optional<Duration> ttl, Optional<Duration> waitFor,
                       Optional<ReplyTo> replyTo, Optional<String> ask) {
        public static Send none() { return new Send(Optional.empty(), Optional.empty(), Optional.empty(), Optional.empty(), Optional.empty(), Optional.empty()); }
        public static Send topic(String t) { return none().withTopic(t); }
        public Send withTopic(String t) { return new Send(Optional.of(t), tag, ttl, waitFor, replyTo, ask); }
        public Send tag(String t) { return new Send(topic, Optional.of(t), ttl, waitFor, replyTo, ask); }
        public Send ttl(Duration d) { return new Send(topic, tag, Optional.of(d), waitFor, replyTo, ask); }
        public Send waitFor(Duration d) { return new Send(topic, tag, ttl, Optional.of(d), replyTo, ask); }
        public Send replyTo(ReplyTo r) { return new Send(topic, tag, ttl, waitFor, Optional.of(r), ask); }
        public Send ask(String a) { return new Send(topic, tag, ttl, waitFor, replyTo, Optional.of(a)); }
    }

    /** A consume: which inbox, which topic and tag, how long to wait (the daemon caps it at 60 s), and whether this reader is one of a pool. */
    public record Consume(Optional<String> inbox, Optional<String> topic, Optional<String> tag, Duration within, boolean share) {
        public static Consume waiting(Duration d) { return new Consume(Optional.empty(), Optional.empty(), Optional.empty(), d, false); }
        public static Consume now() { return waiting(Duration.ZERO); }
        public Consume inbox(String n) { return new Consume(Optional.of(n), topic, tag, within, share); }
        public Consume topic(String t) { return new Consume(inbox, Optional.of(t), tag, within, share); }
        public Consume tag(String t) { return new Consume(inbox, topic, Optional.of(t), within, share); }
        public Consume shared() { return new Consume(inbox, topic, tag, within, true); }
    }

    /** A call: how long the caller waits (also sent as the message's wait), its topic, an inbox only it reads when its own is shared, and an ask as for {@link Send}. */
    public record Call(Duration timeout, String topic, Optional<Duration> ttl, Optional<String> replyTo, Optional<String> ask) {
        public static Call within(Duration d) { return new Call(d, "call", Optional.empty(), Optional.empty(), Optional.empty()); }
        public Call topic(String t) { return new Call(timeout, t, ttl, replyTo, ask); }
        public Call ttl(Duration d) { return new Call(timeout, topic, Optional.of(d), replyTo, ask); }
        /** An inbox the current caller may consume under its ACL; an address alone grants nothing. */
        public Call replyTo(String inbox) { return new Call(timeout, topic, ttl, Optional.of(inbox), ask); }
        public Call ask(String a) { return new Call(timeout, topic, ttl, replyTo, Optional.of(a)); }
    }

    /** Serving: how many handlers at once, which topic, how long a stop waits for running handlers, and whether this process is one of a pool. */
    public record Serve(int workers, Optional<String> topic, Duration drain, boolean share, Duration poll,
                        Optional<java.util.function.BiConsumer<Message, Throwable>> onError) {
        public static Serve workers(int n) {
            if (n < 1) throw new IllegalArgumentException("at least one worker");
            return new Serve(n, Optional.empty(), Duration.ofSeconds(30), false, Duration.ofSeconds(5), Optional.empty());
        }
        public Serve topic(String t) { return new Serve(workers, Optional.of(t), drain, share, poll, onError); }
        public Serve drain(Duration d) { return new Serve(workers, topic, d, share, poll, onError); }
        public Serve shared() { return new Serve(workers, topic, drain, true, poll, onError); }
        /** Each wait's length: a stop is seen at the latest after one. */
        public Serve poll(Duration d) { return new Serve(workers, topic, drain, share, d, onError); }
        /**
         * Where a handler's failure goes. By default only the message id and the
         * exception's type are logged: its message may carry the request or a
         * private value.
         */
        public Serve onError(java.util.function.BiConsumer<Message, Throwable> f) { return new Serve(workers, topic, drain, share, poll, Optional.of(f)); }
    }
}
