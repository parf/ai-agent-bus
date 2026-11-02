package agentbus;

import agentbus.BusException.Busy;
import agentbus.BusException.Exists;
import agentbus.BusException.Forbidden;
import agentbus.BusException.IdentityMismatchException;
import agentbus.BusException.Invalid;
import agentbus.BusException.LeaseExpiredException;
import agentbus.BusException.NotFound;
import agentbus.BusException.RefusalException;
import agentbus.BusException.TransportException;
import agentbus.BusException.Unauthenticated;
import agentbus.BusException.UncertainOutcomeException;
import agentbus.KV.How;
import agentbus.KV.Kind;
import agentbus.KV.Op;
import agentbus.Locks.Lease;
import agentbus.Message.ReplyTo;
import agentbus.Options.Call;
import agentbus.Options.Connect;
import agentbus.Options.Consume;
import agentbus.Options.Send;
import agentbus.Options.Serve;
import agentbus.Server.Response;

import java.net.StandardProtocolFamily;
import java.net.URI;
import java.net.UnixDomainSocketAddress;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.ByteBuffer;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.CancellationException;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Function;
import java.util.function.Supplier;

/**
 * The Java client against a real daemon, run through tests/daemon.sh:
 * {@code src/clients/java/tests/run.sh [section]}.
 */
public final class BusTest {
    static final String ADDR = env("AB_TEST_ADDR"), OWNER = env("AB_TEST_OWNER"), TOKEN = env("AB_TEST_OWNER_TOKEN"),
            DIR = env("AB_TEST_DIR"), TCP = env("AB_TEST_TCP");
    static int pass, fail;
    static String filter = "";

    static String env(String k) {
        String v = System.getenv(k);
        if (v == null) throw new IllegalStateException("run me through tests/daemon.sh: no " + k);
        return v;
    }

    static void check(String what, boolean ok, Object detail) {
        if (ok) { pass++; System.out.println("  ok   " + what); }
        else { fail++; System.out.println("  FAIL " + what + ": " + detail); }
    }

    static void check(String what, boolean ok) { check(what, ok, ""); }

    interface Body { void run() throws Exception; }

    static Throwable thrown(Body f) {
        try { f.run(); } catch (Throwable e) { return e; }
        return null;
    }

    /**
     * The Go fixture daemon, memory-backed like the other clients' state vectors use: one per case, so no case depends
     * on another and a 10,000-name fill costs memory, not SQLite. close() ends it.
     */
    static final class VectorDaemon implements AutoCloseable {
        final Process process;
        final String addr;
        final Map<?, ?> tokens;

        VectorDaemon() throws Exception {
            String binary = System.getenv("AB_CLIENT_FIXTURE");
            ProcessBuilder pb = binary != null && !binary.isEmpty() ? new ProcessBuilder(binary)
                    : new ProcessBuilder("go", "run", "./clients/fixture").directory(Path.of(env("AB_TEST_DIR")).getParent().getParent().resolve("src").toFile());
            pb.redirectError(ProcessBuilder.Redirect.INHERIT);
            process = pb.start();
            String line = new java.io.BufferedReader(new java.io.InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8)).readLine();
            Map<?, ?> info = (Map<?, ?>) Json.parse(line);
            addr = (String) info.get("addr");
            tokens = (Map<?, ?>) info.get("tokens");
        }

        Bus as(String name) {
            return Bus.connect(Connect.of(addr, (String) tokens.get(name)).expectName(name).timeout(Duration.ofSeconds(10)));
        }

        @Override public void close() throws Exception {
            process.getOutputStream().close();
            process.waitFor();
        }
    }

    static Object or(Map<?, ?> m, String key, Object fallback) { Object v = m.get(key); return v == null ? fallback : v; }

    static String name(Throwable e) { return e == null ? "none" : e.getClass().getSimpleName() + " " + e.getMessage(); }

    /** Every expected key is read, so a field added to the shared fixtures cannot go unchecked. */
    static boolean sameFields(Message m, Map<?, ?> want) {
        for (Map.Entry<?, ?> e : want.entrySet()) {
            String got = switch ((String) e.getKey()) {
                case "from" -> m.from();
                case "to" -> m.to();
                case "body" -> m.body();
                case "topic" -> m.topic().orElse("");
                case "tag" -> m.tag().orElse("");
                case "ask" -> m.ask().orElse("");
                default -> "no adapter for expected field " + e.getKey();
            };
            if (!got.equals(e.getValue())) return false;
        }
        return true;
    }

    static final List<String> AGENTS = List.of("#j-svc@localhost", "#j-slow@localhost", "#j-helper@localhost", "#j-other@localhost", "#j-caller@localhost");

    static void awaitReaderRelease(String name) throws Exception {
        // Closing a client socket joins its local reader, not the daemon's
        // cancellation handler. Observe remote release before reusing an inbox.
        long until = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
        while (true) {
            Map<String, Object> record = Json.object(owner("GET", "/lookup?name="
                    + java.net.URLEncoder.encode(name, StandardCharsets.UTF_8), null));
            if (((Number) record.get("readers")).intValue() == 0) return;
            if (System.nanoTime() >= until) throw new AssertionError("reader did not leave " + name);
            Thread.sleep(5);
        }
    }

    static void section(String name, Body f) {
        if (!filter.isEmpty() && !name.contains(filter)) return;
        System.out.println("== " + name);
        try {
            for (String agent : AGENTS) awaitReaderRelease(agent);
            f.run();
        } catch (Throwable e) { check(name + " ran to the end", false, name(e)); e.printStackTrace(System.out); }
    }

    static final HttpClient RAW = HttpClient.newHttpClient();

    /** A raw call as the owner over TCP, for what the library does not write. */
    static String owner(String method, String path, String json) throws Exception {
        HttpRequest.Builder b = HttpRequest.newBuilder(URI.create(TCP + path)).header("X-Agent-Bus-Token", TOKEN)
                .method(method, json == null ? HttpRequest.BodyPublishers.noBody() : HttpRequest.BodyPublishers.ofString(json));
        HttpResponse<String> r = RAW.send(b.build(), HttpResponse.BodyHandlers.ofString());
        if (r.statusCode() >= 400) throw new IllegalStateException(method + " " + path + ": " + r.statusCode() + " " + r.body());
        return r.body();
    }

    static String token(String name) throws Exception {
        return String.valueOf(Json.object(owner("POST", "/token", Json.write(Map.of("name", name)))).get("token"));
    }

    static Bus client(String name) throws Exception {
        return Bus.connect(Connect.of(ADDR, token(name)).expectName(name).timeout(Duration.ofSeconds(10)));
    }

    static Duration ms(long n) { return Duration.ofMillis(n); }

    /** How often the malformed-config fake has been asked: its first answer does not decode. */
    static final AtomicInteger CONFIG_ASKED = new AtomicInteger();

    static boolean awaitQuietly(Server s, long ms) {
        Thread t = Thread.ofVirtual().start(() -> { try { s.awaitTermination(); } catch (InterruptedException ignored) { } });
        try { t.join(ms); } catch (InterruptedException e) { return false; }
        return !t.isAlive();
    }

    static <T> T eventually(Supplier<T> f, java.util.function.Predicate<T> done, long within) throws InterruptedException {
        long until = System.currentTimeMillis() + within;
        T v = f.get();
        while (!done.test(v) && System.currentTimeMillis() < until) { Thread.sleep(20); v = f.get(); }
        return v;
    }


    /** One scripted answer: after delayMs, a status (or -1 to hang up) and a body. */
    record Answer(long delayMs, int status, String body, String headers) {
        Answer(long delayMs, int status, String body) { this(delayMs, status, body, ""); }
    }

    /** A scripted daemon on a unix socket: answers each request as {@code script} says; counts them. */
    static final class Fake implements AutoCloseable {
        final AtomicInteger seen = new AtomicInteger();
        final ServerSocketChannel srv;
        final Thread loop;

        final java.util.List<String> lines = Collections.synchronizedList(new ArrayList<>());

        /** Every request line, the protocol handshake included. */
        final java.util.List<String> all = Collections.synchronizedList(new ArrayList<>());

        /** What GET /identity answers; null hands it to the script like any other request. */
        volatile Answer identity = new Answer(0, 200, "{\"protocol\":1,\"protocol_min\":1}");

        /** On loopback TCP instead of a unix socket, IPv4 or IPv6. */
        static Fake tcp(java.util.function.BiFunction<String, Integer, Answer> scripted) throws Exception { return new Fake(null, scripted); }

        /** The path "::1" stands for loopback IPv6, as null stands for IPv4. */
        static Fake tcp6(java.util.function.BiFunction<String, Integer, Answer> scripted) throws Exception {
            return new Fake(Path.of("::1"), scripted);
        }

        String url() throws Exception {
            java.net.InetSocketAddress a = (java.net.InetSocketAddress) srv.getLocalAddress();
            return a.getAddress() instanceof java.net.Inet6Address ? "http://[::1]:" + a.getPort() : "http://127.0.0.1:" + a.getPort();
        }

        long count(String prefix) { synchronized (lines) { return lines.stream().filter(l -> l.startsWith(prefix)).count(); } }

        Fake(Path sock, java.util.function.BiFunction<String, Integer, Answer> scripted) throws Exception {
            java.util.function.BiFunction<String, Integer, Answer> script = (line, i) ->
                    line.startsWith("GET /status") ? new Answer(0, 200, "{\"you\":\"me@x\"}") : scripted.apply(line, i);
            if (sock == null) {
                srv = ServerSocketChannel.open(StandardProtocolFamily.INET);
                srv.bind(new java.net.InetSocketAddress(java.net.InetAddress.getLoopbackAddress(), 0));
            } else if (sock.toString().equals("::1")) {
                srv = ServerSocketChannel.open(StandardProtocolFamily.INET6);
                srv.bind(new java.net.InetSocketAddress(java.net.InetAddress.getByName("::1"), 0));
            } else {
                Files.deleteIfExists(sock);
                srv = ServerSocketChannel.open(StandardProtocolFamily.UNIX);
                srv.bind(UnixDomainSocketAddress.of(sock));
            }
            loop = Thread.ofVirtual().start(() -> {
                while (true) {
                    SocketChannel c;
                    try { c = srv.accept(); } catch (Exception e) { return; }
                    Thread.ofVirtual().start(() -> {
                        try (c) {
                            ByteBuffer b = ByteBuffer.allocate(65536);
                            StringBuilder req = new StringBuilder();
                            while (!req.toString().contains("\r\n\r\n") && c.read(b) > 0) {
                                req.append(new String(b.array(), 0, b.position(), StandardCharsets.ISO_8859_1));
                                b.clear();
                            }
                            int bodyAt = req.indexOf("\r\n\r\n");
                            if (bodyAt < 0) return;
                            int length = 0;
                            for (String header : req.substring(0, bodyAt).split("\r\n")) {
                                int colon = header.indexOf(':');
                                if (colon >= 0 && header.substring(0, colon).equalsIgnoreCase("Content-Length")) {
                                    length = Integer.parseInt(header.substring(colon + 1).trim());
                                }
                            }
                            // ISO-8859-1 keeps one character per byte, including UTF-8 bodies.
                            // The client writes headers and body separately; replying at the
                            // header boundary can close the socket during its body write.
                            while (req.length() - bodyAt - 4 < length) {
                                if (c.read(b) < 0) return;
                                req.append(new String(b.array(), 0, b.position(), StandardCharsets.ISO_8859_1));
                                b.clear();
                            }
                            String first = req.toString().split("\r\n")[0];
                            all.add(first);
                            // The protocol handshake is answered apart and counted in no test's tally.
                            Answer a;
                            if (first.startsWith("GET /identity") && identity != null) {
                                a = identity;
                            } else {
                                int i = seen.getAndIncrement();
                                lines.add(first);
                                a = script.apply(first, i);
                            }
                            Thread.sleep(a.delayMs());
                            if (a.status() < 0) return;
                            byte[] body = a.body().getBytes(StandardCharsets.UTF_8);
                            c.write(ByteBuffer.wrap(("HTTP/1.1 " + a.status() + " X\r\nContent-Type: application/json\r\nContent-Length: "
                                    + body.length + "\r\n" + a.headers() + "Connection: close\r\n\r\n").getBytes(StandardCharsets.ISO_8859_1)));
                            c.write(ByteBuffer.wrap(body));
                        } catch (Exception ignored) { }
                    });
                }
            });
        }

        @Override public void close() throws Exception { srv.close(); }
    }

    /** The tag a request line asked for, for a fake that answers a call. */
    static String query(String line, String key) {
        String target = line.split(" ")[1];
        int q = target.indexOf('?');
        if (q < 0) return "";
        for (String kv : target.substring(q + 1).split("&")) {
            String[] p = kv.split("=", 2);
            if (p[0].equals(key) && p.length == 2) return java.net.URLDecoder.decode(p[1], StandardCharsets.UTF_8);
        }
        return "";
    }

    public static void main(String[] args) throws Exception {
        if (args.length > 0) filter = args[0];
        owner("POST", "/user", Json.write(Map.of("name", "bob@localhost", "create", true)));
        Bus me = Bus.connect(Connect.of(ADDR, TOKEN).expectName(OWNER).timeout(Duration.ofSeconds(10)));
        for (String n : AGENTS) {
            me.register(Map.of("kind", "agent", "name", n, "allow", List.of("*")));
        }
        me.register(Map.of("kind", "queue", "name", "j-replies@localhost", "allow", List.of(OWNER, "#j-svc@localhost", "#j-caller@localhost")));
        me.register(Map.of("kind", "pubsub", "name", "j-news@localhost", "allow", List.of("*"), "subs", List.of("#j-other@localhost")));

        section("identity", () -> {
            check("the caller is who the credential says", me.name().equals(OWNER), me.name());
            String shown = Connect.of(ADDR, TOKEN).toString();
            check("connection options never print the token", !shown.contains(TOKEN) && shown.contains("token=[redacted]"), shown);
            Throwable e = thrown(() -> Bus.connect(Connect.of(ADDR, TOKEN).expectName("bob@localhost")));
            check("an unexpected name is refused at connect", e instanceof IdentityMismatchException, name(e));
            Map<String, Object> node = me.identity();
            check("identity() is the node, not the caller", node.containsKey("version") && !node.containsKey("you"), node);
            e = thrown(() -> Bus.connect(Connect.of(ADDR, "not-a-token")).list(Optional.empty()));
            check("a bad token is Unauthenticated", e instanceof Unauthenticated, name(e));
            Bus overTcp = Bus.connect(Connect.of(TCP, TOKEN).expectName(OWNER));
            Bus svc = client("#j-svc@localhost");
            String id = overTcp.send("#j-svc@localhost", "over tcp", Send.topic("tcp"));
            Optional<Message> m = svc.consume(Consume.waiting(ms(2000)));
            check("TCP sends as the unix socket does", m.isPresent() && m.get().id().equals(id) && m.get().body().equals("over tcp"), m);
            e = thrown(() -> overTcp.send("nobody@localhost", "x"));
            check("and refuses the same way", e instanceof NotFound, name(e));
        });

        section("send and consume", () -> {
            Bus svc = client("#j-svc@localhost");
            String id = me.send("#j-svc@localhost", "hello", Send.topic("greet").tag("t1").ttl(Duration.ofMinutes(1)));
            Optional<Message> m = svc.consume(Consume.waiting(ms(2000)));
            check("the message arrives with its fields", m.isPresent() && m.get().id().equals(id) && m.get().body().equals("hello")
                    && m.get().topic().orElse("").equals("greet") && m.get().tag().orElse("").equals("t1") && m.get().expires().isPresent(), m);
            check("an empty inbox answers empty", svc.consume(Consume.now()).isEmpty());
            me.send("#j-svc@localhost", "one", Send.topic("a"));
            me.send("#j-svc@localhost", "two", Send.topic("b"));
            check("a topic filter takes the matching one", svc.consume(Consume.waiting(ms(1000)).topic("b")).map(Message::body).orElse("").equals("two"));
            check("the other is still there", svc.consume(Consume.waiting(ms(1000))).map(Message::body).orElse("").equals("one"));
            Throwable zeroWait = thrown(() -> me.send("#j-svc@localhost", "zero", Send.topic("z").waitFor(Duration.ZERO)));
            check("a zero wait is a send without one", zeroWait == null
                    && svc.consume(Consume.waiting(ms(1000))).map(Message::body).orElse("").equals("zero"), String.valueOf(zeroWait));
            Throwable subMs = thrown(() -> me.send("#j-svc@localhost", "sub", Send.topic("z").waitFor(Duration.ofNanos(400_000))));
            check("a wait under a millisecond is a send without one", subMs == null
                    && svc.consume(Consume.waiting(ms(1000))).map(Message::body).orElse("").equals("sub"), String.valueOf(subMs));
            Throwable negWait = thrown(() -> me.send("#j-svc@localhost", "neg", Send.topic("z").waitFor(Duration.ofMillis(-1))));
            check("a negative wait is still refused, not dropped", negWait instanceof BusException.Invalid, name(negWait));
            check("and nothing was delivered", svc.consume(Consume.now()).isEmpty());
            check("roles cannot be changed", thrown(() -> svc.consume(Consume.now())) == null
                    && thrown(() -> new Message("i", "f", "t", Optional.empty(), Optional.empty(), "", java.time.Instant.EPOCH, Optional.empty(),
                    Optional.empty(), Optional.empty(), Optional.empty(), new ArrayList<>(List.of("x")), Optional.empty(), 0, Optional.empty(), Optional.empty()).roles().add("y")) != null);
        });

        section("reply and receipts", () -> {
            Bus svc = client("#j-svc@localhost");
            me.send("#j-svc@localhost", "q", Send.topic("ask").tag("r1"));
            Message m = svc.consume(Consume.waiting(ms(2000))).orElseThrow();
            svc.ack(m);
            svc.reply(m, "a");
            Optional<Message> ack = me.consume(Consume.waiting(ms(2000)).topic("ask").tag("r1"));
            Optional<Message> ans = me.consume(Consume.waiting(ms(2000)).topic("ask").tag("r1"));
            check("the ack names the request", ack.flatMap(Message::receipt).orElse(null) == Message.Receipt.ACK && ack.get().re().orElse("").equals(m.id()), ack);
            check("the reply carries topic, tag and the request", ans.map(Message::body).orElse("").equals("a") && ans.get().receipt().isEmpty()
                    && ans.get().re().orElse("").equals(m.id()), ans);
            me.send("#j-svc@localhost", "q2", Send.topic("ask").tag("r2").replyTo(ReplyTo.of("j-replies@localhost", "back", "r2b")));
            Message m2 = svc.consume(Consume.waiting(ms(2000))).orElseThrow();
            svc.reply(m2, "there");
            check("a reply follows reply_to", me.consume(Consume.waiting(ms(2000)).inbox("j-replies@localhost").topic("back").tag("r2b"))
                    .map(Message::body).orElse("").equals("there"));
        });

        section("call", () -> {
            Bus svc = client("#j-svc@localhost");
            try (Server s = svc.serve((m, ctx) -> switch (m.topic().orElse("")) {
                case "echo" -> Response.reply("echo:" + m.body());
                case "none" -> Response.done();
                case "empty" -> Response.reply("");
                case "boom" -> throw new IllegalStateException("boom");
                default -> Response.reply("via queue");
            }, Serve.workers(2).poll(ms(500)))) {
                Outcome o = me.call("#j-svc@localhost", "hi", Call.within(Duration.ofSeconds(5)).topic("echo"));
                check("an answer is an Answer, acked first", o instanceof Outcome.Answer a && a.message().body().equals("echo:hi") && a.acked(), o);
                o = me.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(5)).topic("none"));
                check("done() is Done", o instanceof Outcome.Done, o);
                o = me.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(5)).topic("empty"));
                check("an empty reply is an answer, not done", o instanceof Outcome.Answer a && a.message().body().isEmpty(), o);
                o = me.call("#j-svc@localhost", "x", Call.within(ms(1500)).topic("boom"));
                check("a throwing handler sends no done: timed out, acked", o instanceof Outcome.TimedOut t && t.acked(), o);
                o = me.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(5)).topic("q").replyTo("j-replies@localhost"));
                check("a call with replyTo is answered in that inbox", o instanceof Outcome.Answer a && a.message().to().equals("j-replies@localhost"), o);
            }
            Outcome o = me.call("#j-slow@localhost", "x", Call.within(ms(500)));
            check("nobody serving: TimedOut, not acked", o instanceof Outcome.TimedOut t && !t.acked(), o);
            client("#j-slow@localhost").consume(Consume.waiting(ms(500)));
            // An interrupt cancels the wait, keeps the flag, and is not a timeout.
            AtomicReference<Throwable> got = new AtomicReference<>();
            AtomicBoolean flag = new AtomicBoolean();
            Thread caller = Thread.ofVirtual().start(() -> {
                got.set(thrown(() -> me.call("#j-slow@localhost", "x", Call.within(Duration.ofSeconds(10)))));
                flag.set(Thread.currentThread().isInterrupted());
            });
            Thread.sleep(300);
            caller.interrupt();
            caller.join(3000);
            check("an interrupted call throws CancellationException", got.get() instanceof CancellationException, name(got.get()));
            check("and keeps the interrupt flag", flag.get());
            client("#j-slow@localhost").consume(Consume.waiting(ms(500)));
        });

        section("ask", () -> {
            Bus svc = client("#j-svc@localhost");
            try (Server s = svc.serve((m, ctx) -> Response.reply("ask=" + m.ask().orElse("") + " body=" + m.body()), Serve.workers(1).poll(ms(500)))) {
                Outcome o = me.call("#j-svc@localhost", "", Call.within(Duration.ofSeconds(5)).ask("help"));
                check("a call can ask: the ask travels as the field, with no body", o instanceof Outcome.Answer a && a.message().body().equals("ask=help body="), o);
                o = me.call("#j-svc@localhost", "work", Call.within(Duration.ofSeconds(5)));
                check("a plain call carries no ask", o instanceof Outcome.Answer a && a.message().body().equals("ask= body=work"), o);
                Throwable e = thrown(() -> me.send("#j-svc@localhost", "text", Send.none().ask("help")));
                check("an ask with a body is refused as invalid", e instanceof Invalid i && i.status() == 400, name(e));
            }
        });

        section("parallel handlers", () -> {
            Bus svc = client("#j-svc@localhost");
            AtomicInteger now = new AtomicInteger(), most = new AtomicInteger();
            try (Server s = svc.serve((m, ctx) -> {
                most.accumulateAndGet(now.incrementAndGet(), Math::max);
                Thread.sleep(300);
                now.decrementAndGet();
                return Response.reply("w");
            }, Serve.workers(3).poll(ms(300)))) {
                List<Thread> ts = new ArrayList<>();
                List<Outcome> outs = Collections.synchronizedList(new ArrayList<>());
                long t0 = System.currentTimeMillis();
                for (int i = 0; i < 9; i++) ts.add(Thread.ofVirtual().start(() -> outs.add(me.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(10))))));
                for (Thread t : ts) t.join();
                long took = System.currentTimeMillis() - t0;
                check("nine calls are all answered", outs.size() == 9 && outs.stream().allMatch(o -> o instanceof Outcome.Answer), outs);
                check("never more than three handlers at once", most.get() == 3, most.get());
                check("and they ran in parallel", took < 2500, took + "ms");
            }
        });

        section("capacity", () -> {
            // The one handler calls out and waits; its reply must still arrive.
            Bus svc = client("#j-svc@localhost"), helper = client("#j-helper@localhost");
            try (Server h = helper.serve((m, ctx) -> Response.reply("helped"), Serve.workers(1).poll(ms(300)));
                 Server s = svc.serve((m, ctx) -> {
                     Outcome o = svc.call("#j-helper@localhost", "x", Call.within(Duration.ofSeconds(5)));
                     return Response.reply(o instanceof Outcome.Answer a ? a.message().body() : o.toString());
                 }, Serve.workers(1).poll(ms(300)))) {
                Outcome o = me.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(8)));
                check("a handler's own call is answered while the pool is full", o instanceof Outcome.Answer a && a.message().body().equals("helped"), o);
            }
        });

        section("stray replies", () -> {
            Bus caller = client("#j-caller@localhost"), svc = client("#j-svc@localhost");
            try (Server h = svc.serve((m, ctx) -> Response.reply("strayed"), Serve.workers(1).poll(ms(300)))) {
                caller.pollReplies = false;
                // Only the unfiltered serve reader can see the reply now.
                try (Server s = caller.serve((m, ctx) -> Response.reply("not a request"), Serve.workers(2).poll(ms(300)))) {
                    Outcome o = caller.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(5)));
                    check("serve's reader hands a stray reply to its call", o instanceof Outcome.Answer a && a.message().body().equals("strayed"), o);
                }
                awaitReaderRelease("#j-caller@localhost");
                AtomicReference<Outcome> out = new AtomicReference<>();
                Thread t = Thread.ofVirtual().start(() -> out.set(caller.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(5)))));
                Optional<Message> taken = caller.consume(Consume.waiting(ms(2500)));
                t.join(5000);
                check("so does consume(), and goes on waiting", taken.isEmpty() && out.get() instanceof Outcome.Answer, taken + " " + out.get());
                caller.pollReplies = true;
            }
        });

        section("shared send and call cases", () -> {
            // The shared cases, with their fixture names mapped to this daemon's.
            Function<String, List<?>> load = file -> {
                try {
                    Path cases = Path.of(env("AB_CLIENT_CONFORMANCE_CASES"), file + ".json");
                    List<?> all = (List<?>) Json.parse(Files.readString(cases).replace("#caller@fixture", OWNER).replace("#worker@fixture", "#j-svc@localhost"));
                    check("the shared " + file + " cases are not empty", !all.isEmpty(), cases);
                    return all;
                } catch (java.io.IOException e) {
                    throw new java.io.UncheckedIOException(e);
                }
            };
            Bus svc = client("#j-svc@localhost");
            for (Object c : load.apply("send")) {
                Map<?, ?> k = (Map<?, ?>) c, req = (Map<?, ?>) k.get("request"), want = (Map<?, ?>) k.get("expected");
                Send options = Send.none();
                if (req.get("topic") != null) options = options.withTopic((String) req.get("topic"));
                if (req.get("tag") != null) options = options.tag((String) req.get("tag"));
                if (req.get("ask") != null) options = options.ask((String) req.get("ask"));
                String id = me.send((String) req.get("to"), (String) req.get("body"), options);
                Optional<Message> got = svc.consume(Consume.waiting(ms(2000)));
                check("send " + k.get("id") + ": arrives field for field under the returned ID", got.isPresent()
                        && got.get().id().equals(id) && sameFields(got.get(), want), got);
            }
            List<?> calls = load.apply("call");
            List<String> handled = new java.util.concurrent.CopyOnWriteArrayList<>();
            try (Server s = svc.serve((m, ctx) -> {
                for (Object c : calls) {
                    Map<?, ?> k = (Map<?, ?>) c;
                    if (((Map<?, ?>) k.get("request")).get("body").equals(m.body())) {
                        handled.add(m.id() + " " + m.body());
                        return Response.reply((String) k.get("answer"));
                    }
                }
                return Response.reply("no case for " + m.body());
            }, Serve.workers(1).poll(ms(300)))) {
                for (Object c : calls) {
                    Map<?, ?> k = (Map<?, ?>) c, req = (Map<?, ?>) k.get("request"), want = (Map<?, ?>) k.get("expected");
                    Outcome o = me.call((String) req.get("to"), (String) req.get("body"),
                            Call.within(Duration.ofSeconds(5)).topic((String) req.get("topic")));
                    check("call " + k.get("id") + ": the handler's answer", o instanceof Outcome.Answer a
                            && sameFields(a.message(), want), o);
                    check("call " + k.get("id") + ": the handler ran once on the request it answers", o instanceof Outcome.Answer a
                            && handled.stream().filter(h -> h.equals(a.message().re().orElse("") + " " + req.get("body"))).count() == 1, handled + " " + o);
                }
            }
            check("each call ran its handler once", handled.size() == calls.size(), handled);
        });

        section("self call", () -> {
            // The shared cases, with their fixture names mapped to this daemon's.
            Path cases = Path.of(env("AB_CLIENT_CONFORMANCE_CASES"), "self-call.json");
            List<?> all = (List<?>) Json.parse(Files.readString(cases).replace("#caller@fixture", "#j-caller@localhost"));
            check("the shared self-call cases are not empty", !all.isEmpty(), cases);
            for (Object c : all) {
                Map<?, ?> k = (Map<?, ?>) c, req = (Map<?, ?>) k.get("request"), want = (Map<?, ?>) k.get("expected");
                String to = (String) req.get("to"), topic = (String) req.get("topic");
                for (String replyTo : new String[] {null, "j-replies@localhost"}) {
                    Bus self = client(to);
                    List<String> handled = new java.util.concurrent.CopyOnWriteArrayList<>();
                    Outcome o;
                    try (Server s = self.serve((m, ctx) -> {
                        handled.add(m.id() + " " + m.body());
                        return Response.reply((String) k.get("answer"));
                    }, Serve.workers(2).poll(ms(300)))) {
                        Call call = Call.within(Duration.ofSeconds(5)).topic(topic);
                        o = self.call(to, (String) req.get("body"), replyTo == null ? call : call.replyTo(replyTo));
                    }
                    String what = k.get("id") + (replyTo == null ? "" : " via " + replyTo);
                    check(what + ": the handler's answer, not the request", o instanceof Outcome.Answer a
                            && a.message().body().equals(want.get("body")) && a.message().from().equals(want.get("from"))
                            && a.message().to().equals(replyTo == null ? want.get("to") : replyTo)
                            && a.message().topic().orElse("").equals(want.get("topic")), o);
                    check(what + ": the handler ran once on the request it answers", handled.size() == 1 && o instanceof Outcome.Answer a
                            && handled.get(0).equals(a.message().re().orElse("") + " " + req.get("body")), handled + " " + o);
                    self.close();
                    awaitReaderRelease(to);
                }
            }
        });

        section("serve rules", () -> {
            Bus svc = client("#j-svc@localhost");
            List<String> ran = Collections.synchronizedList(new ArrayList<>());
            me.send("#j-svc@localhost", "late", Send.topic("late").tag("l1").waitFor(ms(50)));
            owner("POST", "/send", Json.write(Map.of("to", "#j-svc@localhost", "body", "a late answer", "re", "earlier")));
            Thread.sleep(200);
            try (Server s = svc.serve((m, ctx) -> { ran.add(m.body()); return Response.none(); }, Serve.workers(1).poll(ms(300)))) {
                Thread.sleep(1200);
                check("a too-late request is not run", !ran.contains("late"), ran);
                check("an answer arriving late is no request", !ran.contains("a late answer"), ran);
                check("consume() while serving is refused", thrown(() -> svc.consume(Consume.now())) instanceof IllegalStateException);
            }
            check("nor was the late one acked", me.consume(Consume.now().topic("late").tag("l1")).isEmpty());
            // close() waits for a running handler, up to the drain.
            CountDownLatch started = new CountDownLatch(1);
            AtomicBoolean finished = new AtomicBoolean(), answered = new AtomicBoolean();
            Server s = svc.serve((m, ctx) -> { started.countDown(); Thread.sleep(500); finished.set(true); return Response.reply("drained"); },
                    Serve.workers(1).poll(ms(300)).drain(Duration.ofSeconds(3)));
            Thread caller = Thread.ofVirtual().start(() -> {
                Outcome o = me.call("#j-svc@localhost", "x", Call.within(Duration.ofSeconds(5)));
                if (o instanceof Outcome.Answer a && a.message().body().equals("drained")) answered.set(true);
            });
            started.await(5, TimeUnit.SECONDS);
            s.close();
            caller.join(5000);
            check("close() lets a running handler finish and answer", finished.get() && answered.get());
            // Past the drain the handler is cancelled and interrupted.
            CountDownLatch began = new CountDownLatch(1);
            AtomicBoolean sawCancel = new AtomicBoolean(), interrupted = new AtomicBoolean();
            // Two workers, so the reader is in a long poll when the stop comes: the
            // drain still waits for a running handler.
            CountDownLatch going = new CountDownLatch(1);
            AtomicBoolean done2 = new AtomicBoolean();
            Server busy = svc.serve((m, ctx) -> { going.countDown(); Thread.sleep(500); done2.set(true); return Response.none(); },
                    Serve.workers(2).poll(Duration.ofSeconds(5)).drain(Duration.ofSeconds(3)));
            me.send("#j-svc@localhost", "work");
            going.await(5, TimeUnit.SECONDS);
            Thread.sleep(100);
            busy.close();
            check("close() with the reader in a long poll still lets a running handler finish", done2.get());
            // Two workers, so the reader is in a long poll when the stop comes.
            Server s2 = svc.serve((m, ctx) -> {
                began.countDown();
                try { Thread.sleep(10000); } catch (InterruptedException e) { interrupted.set(true); sawCancel.set(ctx.cancelled()); }
                return Response.none();
            }, Serve.workers(2).poll(Duration.ofSeconds(5)).drain(ms(200)));
            me.send("#j-svc@localhost", "slow");
            began.await(5, TimeUnit.SECONDS);
            long t0 = System.currentTimeMillis();
            s2.close();
            check("past the drain a handler is interrupted, its context cancelled", interrupted.get() && sawCancel.get()
                    && System.currentTimeMillis() - t0 < 3000, interrupted.get() + " " + sawCancel.get());
        });

        section("publish", () -> {
            Bus other = client("#j-other@localhost");
            me.publish("j-news@localhost", "extra", Send.none());
            Optional<Message> m = other.consume(Consume.waiting(ms(2000)));
            check("a publication reaches the Deliver-To list", m.map(Message::body).orElse("").equals("extra")
                    && m.get().originalTo().orElse("").equals("j-news@localhost"), m);
            other.unsubscribe("j-news@localhost");
            check("after unsubscribe nobody takes it", thrown(() -> me.publish("j-news@localhost", "gone", Send.none())) instanceof NotFound);
        });

        section("private values", () -> {
            owner("POST", "/configure", Json.write(Map.of("name", "#j-svc@localhost", "config", new Json.Raw("{\"a\":1,\"empty\":{},\"big\":9007199254740993}"))));
            owner("POST", "/secret", Json.write(Map.of("name", "#j-svc@localhost", "secret", "API_KEY=s3cr3t-value\nOTHER='two words'\n")));
            Bus svc = client("#j-svc@localhost");
            Record r = svc.self();
            long before = svc.t.sent.get();
            List<Thread> ts = new ArrayList<>();
            List<String> seen = Collections.synchronizedList(new ArrayList<>());
            CountDownLatch go = new CountDownLatch(1);
            for (int i = 0; i < 16; i++) ts.add(Thread.ofVirtual().start(() -> {
                try { go.await(); } catch (InterruptedException e) { return; }
                seen.add(r.config().orElse(""));
            }));
            go.countDown();
            for (Thread t : ts) t.join();
            check("sixteen first readers share one fetch", svc.t.sent.get() - before == 1, svc.t.sent.get() - before);
            check("and all read it", seen.size() == 16 && seen.stream().allMatch(s -> s.contains("\"a\":1")), seen);
            Map<?, ?> tree = (Map<?, ?>) r.configTree().orElseThrow();
            check("a big int survives", Long.valueOf(9007199254740993L).equals(tree.get("big")), tree.get("big"));
            check("{} stays an object", tree.get("empty") instanceof Map<?, ?> e && e.isEmpty());
            check("the tree cannot be changed", thrown(() -> ((Map<String, Object>) tree).put("a", 2)) instanceof UnsupportedOperationException);
            owner("POST", "/configure", Json.write(Map.of("name", "#j-svc@localhost", "config", Map.of("a", 2))));
            check("the cache is kept until refresh", r.config().orElse("").contains("\"a\":1"));
            r.refresh();
            check("refresh reads it again", r.config().orElse("").contains("\"a\":2"));
            Secret one = r.secret(), two = r.secret();
            check("the secret is its bytes", new String(one.bytes(), StandardCharsets.UTF_8).equals("API_KEY=s3cr3t-value\nOTHER='two words'\n"));
            check("and its env", one.env().equals(Map.of("API_KEY", "s3cr3t-value", "OTHER", "two words")), one.env());
            check("it does not print itself", !String.valueOf(one).contains("s3cr3t") && !List.of(one).toString().contains("s3cr3t"));
            one.close();
            check("closing one copy leaves another", new String(two.bytes(), StandardCharsets.UTF_8).startsWith("API_KEY=s3cr3t"));
            check("and the cache", new String(r.secret().bytes(), StandardCharsets.UTF_8).startsWith("API_KEY=s3cr3t"));
            check("a closed one refuses", thrown(one::bytes) instanceof IllegalStateException);
            byte[] copy = two.bytes();
            copy[0] = 'X';
            check("bytes() is a copy", two.bytes()[0] == 'A');
            check("an absent config is empty", me.record("#j-slow@localhost").config().isEmpty());
            check("an absent secret is NotFound", thrown(() -> me.record("#j-slow@localhost").secret()) instanceof NotFound);
            Bus bob = client("bob@localhost");
            Throwable e = thrown(() -> bob.record("#j-svc@localhost").config());
            check("the allow list alone does not grant it", e instanceof Forbidden, name(e));
            e = thrown(() -> me.record("j-news@localhost").config());
            check("a kind holding no private values is a 400", e instanceof Invalid, name(e));
        });

        section("kv", () -> {
            KV kv = me.record("#j-svc@localhost").kv();
            kv.set("s", "plain text");
            kv.set("n", 41L);
            kv.setJson("j", "{\"jobs\":[],\"meta\":{}}", How.SET);
            check("a string comes back", kv.getString("s").equals("plain text"));
            check("an int is a long", kv.getInt("n") == 41L);
            check("JSON comes back as its text", kv.getJson("j").equals("{\"jobs\":[],\"meta\":{}}"), kv.getJson("j"));
            check("kinds are separate names", thrown(() -> kv.getString("n")) instanceof NotFound);
            byte[] raw = {(byte) 0xff, 0, (byte) 0xfe, 'r'};
            kv.set("b", raw, How.SET);
            check("bytes that are not UTF-8 survive", Arrays.equals(kv.getBytes("b"), raw));
            check("inc answers the new value", kv.inc("n", 1) == 42 && kv.inc("n", -2) == 40);
            check("int64 at its edge survives", thrown(() -> kv.set("max", Long.MAX_VALUE)) == null && kv.getInt("max") == Long.MAX_VALUE);
            Throwable e = thrown(() -> kv.set("s", "again", How.ADD));
            check("add refuses a present name: Exists", e instanceof Exists, name(e));
            e = thrown(() -> kv.set("nope", "x", How.REPLACE));
            check("replace refuses an absent one: NotFound", e instanceof NotFound, name(e));
            List<KV.OpResult> rs = kv.json("j", Op.push("jobs", "a"), Op.push("jobs", "b"), Op.inc("count", 3), Op.shift("jobs"));
            check("ops report what they did", rs.size() == 4 && "\"a\"".equals(rs.get(3).value()) && "3".equals(rs.get(2).value()), rs);
            check("and applied in order", kv.getJson("j").contains("\"jobs\":[\"b\"]"), kv.getJson("j"));
            e = thrown(() -> kv.json("j", Op.push("more", 1), Op.inc("jobs", 1)));
            check("one bad op refuses the list", e instanceof RefusalException, name(e));
            check("and applies none of it", !kv.getJson("j").contains("more"));
            Map<Kind, List<KV.Entry>> l = kv.list();
            check("list answers by kind", l.get(Kind.STRING).stream().anyMatch(x -> x.name().equals("s"))
                    && l.get(Kind.INT).stream().anyMatch(x -> x.name().equals("n")), l);
            check("delete says whether there was one", kv.delete("s", Kind.STRING) && !kv.delete("s", Kind.STRING));
            e = thrown(() -> client("bob@localhost").record("#j-svc@localhost").kv().getInt("n"));
            check("the allow list does not grant the store", e instanceof Forbidden || e instanceof NotFound, name(e));
        });

        section("locks", () -> {
            Locks locks = me.record("#j-svc@localhost").locks();
            Lease first = locks.acquire("build", Duration.ofSeconds(30), Duration.ZERO);
            check("a held lock is listed", locks.holders().getOrDefault("build", new Locks.Hold("", null)).holder().equals(OWNER));
            // A ttl or wait the daemon would refuse is refused here, before any request: ttl positive and at most 24 h, wait at most 60 s.
            for (Object[] edge : new Object[][] {{"ttl 0", (java.util.concurrent.Callable<Object>) () -> locks.tryAcquire("edge", Duration.ZERO)},
                    {"ttl over 24 h", (java.util.concurrent.Callable<Object>) () -> locks.tryAcquire("edge", Duration.ofSeconds(86401))},
                    {"negative ttl", (java.util.concurrent.Callable<Object>) () -> locks.tryAcquire("edge", Duration.ofSeconds(-1))},
                    {"wait over 60 s", (java.util.concurrent.Callable<Object>) () -> locks.acquire("edge", Duration.ofSeconds(30), Duration.ofSeconds(61))}}) {
                Throwable e = thrown(() -> ((java.util.concurrent.Callable<Object>) edge[1]).call());
                check(edge[0] + " is refused by the client", e instanceof IllegalArgumentException, name(e));
            }
            try (Lease own = locks.tryAcquire("again", Duration.ofSeconds(30))) {
                Throwable e = thrown(() -> locks.tryAcquire("again", Duration.ofSeconds(30)));
                check("this name taking its own lock again is Busy and says to extend it", e instanceof Busy && e.getMessage().contains("extend"), name(e));
            }
            try (Lease edge = locks.tryAcquire("edge-ok", Duration.ofHours(24))) {
                check("a ttl of exactly 24 h is accepted", edge.release());
            }
            // A second thread of this Bus queues behind the first rather than being refused.
            AtomicReference<Throwable> got = new AtomicReference<>();
            AtomicBoolean had = new AtomicBoolean();
            Thread second = Thread.ofVirtual().start(() -> got.set(thrown(() -> {
                try (Lease l = locks.acquire("build", Duration.ofSeconds(30), Duration.ofSeconds(5))) { had.set(true); }
            })));
            Thread.sleep(400);
            check("the second thread waits", !had.get() && got.get() == null);
            first.close();
            second.join(5000);
            check("and gets it once the first lets go", had.get() && got.get() == null, name(got.get()));
            Bus svc = client("#j-svc@localhost");
            try (Lease theirs = svc.self().locks().acquire("taken", Duration.ofSeconds(30), Duration.ZERO)) {
                Throwable e = thrown(() -> locks.tryAcquire("taken", Duration.ofSeconds(30)));
                check("held by somebody else: Busy", e instanceof Busy, name(e));
                check("force release answers the displaced holder", locks.forceRelease("taken").orElse("").equals("#j-svc@localhost"));
                check("nobody holds it now", !locks.holders().containsKey("taken"));
                check("its old lease says so on release", thrown(theirs::release) instanceof NotFound);
            }
            Lease shortOne = locks.acquire("short", ms(300), Duration.ZERO);
            Thread.sleep(400);
            check("past its ttl a lease refuses to extend", thrown(() -> shortOne.extend(Duration.ofSeconds(5))) instanceof LeaseExpiredException);
            AtomicReference<Boolean> sent = new AtomicReference<>();
            Throwable e = thrown(() -> sent.set(shortOne.release()));
            check("and to release, sending nothing", e == null && Boolean.FALSE.equals(sent.get()), name(e));
            e = thrown(() -> locks.release("never"));
            check("releasing what nobody holds is NotFound", e instanceof NotFound, name(e));
            Lease ext = locks.acquire("ext", Duration.ofSeconds(1), Duration.ZERO);
            ext.extend(Duration.ofSeconds(30));
            Thread.sleep(1200);
            check("extend keeps it past the first ttl", locks.holders().containsKey("ext"));
            ext.close();
            Bus other = client("#j-svc@localhost");
            Lease held = other.self().locks().acquire("waited", ms(800), Duration.ZERO);
            long t0 = System.currentTimeMillis();
            try (Lease l = locks.acquire("waited", Duration.ofSeconds(5), Duration.ofSeconds(5))) {
                check("another name waits at the daemon and gets it when the hold ends", System.currentTimeMillis() - t0 >= 400);
            }
            held.close();
        });

        section("uncertain writes", () -> {
            Path sock = Path.of(DIR, "j-dead.sock");
            Files.deleteIfExists(sock);
            ServerSocketChannel srv = ServerSocketChannel.open(StandardProtocolFamily.UNIX);
            srv.bind(UnixDomainSocketAddress.of(sock));
            Thread t = Thread.ofVirtual().start(() -> {
                // The first connection is the protocol handshake, answered; the next two are cut off.
                for (int i = 0; i < 3; i++) {
                    try (SocketChannel c = srv.accept()) {
                        c.read(ByteBuffer.allocate(65536));
                        if (i == 0) {
                            String body = "{\"protocol\":1,\"protocol_min\":1}";
                            c.write(ByteBuffer.wrap(("HTTP/1.1 200 X\r\nContent-Type: application/json\r\nContent-Length: "
                                    + body.length() + "\r\nConnection: close\r\n\r\n" + body).getBytes(StandardCharsets.ISO_8859_1)));
                        }
                    } catch (Exception ignored) { return; }
                }
            });
            Bus b = Bus.connect(Connect.of(sock.toString(), "x").timeout(Duration.ofSeconds(2)));
            Throwable e = thrown(() -> b.send("#a@localhost", "x"));
            check("a send whose answer never came is UncertainOutcome", e instanceof UncertainOutcomeException, name(e));
            e = thrown(() -> b.consume(Consume.now()));
            check("a consume cut off mid-answer is uncertain too", e instanceof UncertainOutcomeException, name(e));
            t.join(3000);
            srv.close();
            e = thrown(() -> Bus.connect(Connect.of(DIR + "/none.sock", "x")).send("#a@localhost", "x"));
            check("no daemon at all is Transport: nothing left", e instanceof TransportException, name(e));
        });


        section("json", () -> {
            for (String bad : List.of("01", "1.", "1.e2", ".1", "\"a\nb\"", "{\"a\":1,\"a\":2}", "[1,]", "NaN", "1 2", "{'a':1}", "[1]//", "")) {
                check("JSON refuses " + bad.replace("\n", "\\n"), thrown(() -> Json.parse(bad)) instanceof IllegalArgumentException);
            }
            check("big integers stay exact", Json.parse("123456789012345678901234567890").equals(new java.math.BigInteger("123456789012345678901234567890")));
            check("decimals stay exact", Json.parse("0.1").equals(new java.math.BigDecimal("0.1")));
            check("escapes and unicode round-trip", Json.parse(Json.write("a\"\\\né€")).equals("a\"\\\né€"));
            check("NaN is never written", thrown(() -> Json.write(Double.NaN)) instanceof IllegalArgumentException
                    && thrown(() -> Json.write(Float.POSITIVE_INFINITY)) instanceof IllegalArgumentException);
            check("raw JSON must be JSON", thrown(() -> new Json.Raw("{bad")) instanceof IllegalArgumentException);
        });

        section("fake request framing", () -> {
            Path sock = Path.of(DIR, "j-framing.sock");
            CountDownLatch handled = new CountDownLatch(1);
            try (Fake f = new Fake(sock, (line, i) -> {
                handled.countDown();
                return new Answer(0, 200, "{}");
            }); SocketChannel c = SocketChannel.open(StandardProtocolFamily.UNIX)) {
                c.connect(UnixDomainSocketAddress.of(sock));
                byte[] body = "é".getBytes(StandardCharsets.UTF_8);
                ByteBuffer headers = ByteBuffer.wrap(("POST /send HTTP/1.1\r\nContent-Length: "
                        + body.length + "\r\n\r\n").getBytes(StandardCharsets.ISO_8859_1));
                while (headers.hasRemaining()) c.write(headers);
                boolean early = handled.await(100, TimeUnit.MILLISECONDS);
                check("the fake does not answer a POST before its body arrives", !early);
                if (early) return;
                c.write(ByteBuffer.wrap(body, 0, 1));
                early = handled.await(100, TimeUnit.MILLISECONDS);
                check("the fake waits for every Content-Length byte", !early);
                if (early) return;
                c.write(ByteBuffer.wrap(body, 1, body.length - 1));
                check("the fake handles the completed fragmented POST", handled.await(2, TimeUnit.SECONDS));
            }
        });

        section("budgets and receipts", () -> {
            Path sock = Path.of(DIR, "j-fake.sock");
            String sent = "{\"message_id\":\"m1\"}";
            // Every fake answers who the caller is: a call reads its own inbox name first.
            try (Fake f = new Fake(sock, (line, i) -> line.startsWith("POST /send") ? new Answer(600, 200, sent) : new Answer(0, 204, ""))) {
                long t0 = System.currentTimeMillis();
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).call("#a@x", "b", Call.within(ms(150))));
                long took = System.currentTimeMillis() - t0;
                check("a send slower than the call budget ends at the budget", took < 450, took + "ms");
                check("as an uncertain write", e instanceof UncertainOutcomeException, name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> line.startsWith("POST /send") ? new Answer(0, 200, sent)
                    : new Answer(600, 200, Json.write(Map.of("message_id", "a1", "from", "#a@x", "to", "me", "body", "late",
                    "topic", query(line, "topic"), "tag", query(line, "tag"), "re", "m1"))))) {
                long t0 = System.currentTimeMillis();
                Outcome o = Bus.connect(Connect.of(sock.toString(), "x")).call("#a@x", "b", Call.within(ms(150)));
                long took = System.currentTimeMillis() - t0;
                check("an answer later than the budget is no answer", o instanceof Outcome.TimedOut, o);
                check("and the call ends at its budget", took < 450, took + "ms");
            }
            try (Fake f = new Fake(sock, (line, i) -> line.startsWith("POST /send") ? new Answer(0, 200, sent) : new Answer(0, -1, ""))) {
                long t0 = System.currentTimeMillis();
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).call("#a@x", "b", Call.within(Duration.ofSeconds(2))));
                check("a wait lost early is UncertainOutcome, not TimedOut", e instanceof UncertainOutcomeException
                        && System.currentTimeMillis() - t0 < 1000, name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, sent))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).call("#a@x", "b", Call.within(Duration.ZERO)));
                check("a call with no time is refused before anything is sent", e instanceof IllegalArgumentException && f.seen.get() == 0,
                        name(e) + " " + f.seen.get());
            }
            java.util.function.Function<String, java.util.function.BiFunction<String, Integer, Answer>> receipt = re -> (line, i) -> {
                if (line.startsWith("POST /send")) return new Answer(0, 200, sent);
                Map<String, Object> e = new java.util.LinkedHashMap<>(Map.of("message_id", "r" + i, "from", "#a@x", "to", "me", "body", "",
                        "receipt", "done", "topic", query(line, "topic"), "tag", query(line, "tag")));
                if (re != null) e.put("re", re);
                return new Answer(20, 200, Json.write(e));
            };
            for (String re : new String[] {null, "other"}) {
                try (Fake f = new Fake(sock, receipt.apply(re))) {
                    Outcome o = Bus.connect(Connect.of(sock.toString(), "x")).call("#a@x", "b", Call.within(ms(300)));
                    check("a done receipt naming " + (re == null ? "no message" : "another message") + " is not this call's", o instanceof Outcome.TimedOut, o);
                }
            }
            try (Fake f = new Fake(sock, receipt.apply("m1"))) {
                Outcome o = Bus.connect(Connect.of(sock.toString(), "x")).call("#a@x", "b", Call.within(Duration.ofSeconds(2)));
                check("one naming this call's message is", o instanceof Outcome.Done, o);
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, -1, ""))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).consume(Consume.now()));
                check("a consume that fails after leaving is uncertain: it may have taken one", e instanceof UncertainOutcomeException, name(e));
            }
        });

        section("one request each", () -> {
            // A connection that drops before the answer: the consume is not sent again behind the caller's back.
            try (Fake f = Fake.tcp((line, i) -> new Answer(0, -1, ""))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(f.url(), "x")).consume(Consume.now()));
                Thread.sleep(200);
                check("one consume over TCP is one request, even when the connection drops", e instanceof UncertainOutcomeException
                        && f.count("GET /consume") == 1, name(e) + " " + f.lines);
            }
            try (Fake f = Fake.tcp((line, i) -> new Answer(0, -1, ""))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(f.url(), "x")).send("#a@x", "b"));
                Thread.sleep(200);
                check("so is one send", e instanceof UncertainOutcomeException && f.count("POST /send") == 1, name(e) + " " + f.lines);
            }
            // An IPv6 address is reached as an IPv4 one is.
            try (Fake f = Fake.tcp6((line, i) -> new Answer(0, 200, "{}"))) {
                AtomicReference<String> you = new AtomicReference<>();
                Throwable e = thrown(() -> you.set(Bus.connect(Connect.of(f.url(), "x")).name()));
                check("an IPv6 daemon address works", e == null && "me@x".equals(you.get()), name(e) + " " + f.url());
            }
            // A server whose inbox read is lost mid-answer stops rather than read on.
            Path sock = Path.of(DIR, "j-srv.sock");
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, -1, ""))) {
                Server s = Bus.connect(Connect.of(sock.toString(), "x")).serve((m, ctx) -> Response.none(), Serve.workers(1).poll(ms(200)));
                boolean ended = awaitQuietly(s, 3000);
                Thread.sleep(300);
                check("an inbox read lost mid-answer ends the server", ended && s.failure().orElse(null) instanceof UncertainOutcomeException, s.failure());
                check("after exactly one read", f.count("GET /consume") == 1, f.lines);
            }
            // A configuration that does not decode is refused and not cached.
            Path cfg = Path.of(DIR, "j-badcfg.sock");
            try (Fake f = new Fake(cfg, (line, i) -> line.startsWith("GET /config")
                    ? (CONFIG_ASKED.incrementAndGet() == 1 ? new Answer(0, 200, "{bad") : new Answer(0, 200, "{\"ok\":true}")) : new Answer(0, 404, "{}"))) {
                Record r = Bus.connect(Connect.of(cfg.toString(), "x")).record("#a@x");
                Throwable first = thrown(r::config);
                AtomicReference<Optional<String>> second = new AtomicReference<>();
                Throwable e = thrown(() -> second.set(r.config()));
                check("a config that does not decode is not cached", first instanceof IllegalArgumentException && e == null
                        && second.get().orElse("").contains("ok"), name(first) + " / " + name(e) + " " + second.get());
                check("the second read asked again", f.count("GET /config") == 2, f.lines);
            }
        });

        section("failures surface", () -> {
            Bus bob = client("bob@localhost");
            long t0 = System.currentTimeMillis();
            Throwable e = thrown(() -> bob.call("#j-slow@localhost", "x", Call.within(Duration.ofSeconds(5)).replyTo("j-replies@localhost")));
            check("a call whose wait is refused throws that refusal at once", e instanceof Forbidden && System.currentTimeMillis() - t0 < 2000,
                    name(e) + " " + (System.currentTimeMillis() - t0) + "ms");
            client("#j-slow@localhost").consume(Consume.waiting(ms(300)));
            Bus revoked = Bus.connect(Connect.of(ADDR, "no-such-token"));
            Server s = revoked.serve((m, ctx) -> Response.none(), Serve.workers(1).poll(ms(200)));
            boolean ended = s.failure().isPresent() || awaitQuietly(s, 3000);
            check("a server whose reads are refused stops", ended);
            check("and says why", s.failure().orElse(null) instanceof Unauthenticated, s.failure());
            // The default log names a handler's failure by type, never by its message.
            java.util.logging.Logger jul = java.util.logging.Logger.getLogger("agentbus");
            List<String> lines = Collections.synchronizedList(new ArrayList<>());
            java.util.logging.Handler h = new java.util.logging.Handler() {
                @Override public void publish(java.util.logging.LogRecord r) { lines.add(r.getMessage()); }
                @Override public void flush() { }
                @Override public void close() { }
            };
            jul.addHandler(h);
            Bus svc = client("#j-svc@localhost");
            try (Server t = svc.serve((m, ctx) -> { throw new IllegalStateException("boom-secret"); }, Serve.workers(1).poll(ms(300)))) {
                me.call("#j-svc@localhost", "x", Call.within(ms(800)));
            }
            jul.removeHandler(h);
            String log = String.join("|", lines);
            check("the default log names the failure, not its message", log.contains("IllegalStateException") && !log.contains("boom-secret"), log);
            awaitReaderRelease("#j-svc@localhost");
            List<Throwable> got = Collections.synchronizedList(new ArrayList<>());
            try (Server t = svc.serve((m, ctx) -> { throw new IllegalStateException("for the caller"); },
                    Serve.workers(1).poll(ms(300)).onError((m, err) -> got.add(err)))) {
                me.call("#j-svc@localhost", "x", Call.within(ms(800)));
            }
            check("onError hears the failure itself", got.size() == 1 && "for the caller".equals(got.get(0).getMessage()), got);
        });

        section("close", () -> {
            // A refresh while a read is in flight keeps that read's answer out of the cache.
            Path sock = Path.of(DIR, "j-cfg.sock");
            AtomicInteger asked = new AtomicInteger();
            try (Fake f = new Fake(sock, (line, i) -> line.startsWith("GET /config")
                    ? (asked.incrementAndGet() == 1 ? new Answer(300, 200, "{\"v\":1}") : new Answer(0, 200, "{\"v\":2}"))
                    : new Answer(0, 404, "{}"))) {
                Record cr = Bus.connect(Connect.of(sock.toString(), "x")).record("#a@x");
                Thread reader = Thread.ofVirtual().start(cr::config);
                Thread.sleep(100);
                cr.refresh();
                reader.join(2000);
                check("a refresh during a read keeps the old answer out of the cache", cr.config().orElse("").contains("\"v\":2") && asked.get() == 2,
                        cr.config() + " " + asked.get());
            }
            // close() wakes what is waiting, not only what comes after.
            Bus w = client("#j-caller@localhost");
            AtomicReference<Throwable> callEnd = new AtomicReference<>(), takeEnd = new AtomicReference<>();
            Thread caller = Thread.ofVirtual().start(() -> callEnd.set(thrown(() -> w.call("#j-slow@localhost", "x", Call.within(Duration.ofSeconds(8))))));
            Thread taker = Thread.ofVirtual().start(() -> takeEnd.set(thrown(() -> w.consume(Consume.waiting(Duration.ofSeconds(8)).topic("never")))));
            Thread.sleep(300);
            long t0 = System.currentTimeMillis();
            w.close();
            caller.join(3000);
            taker.join(3000);
            long took = System.currentTimeMillis() - t0;
            check("close wakes a waiting call", callEnd.get() instanceof BusException && took < 1500, name(callEnd.get()) + " " + took + "ms");
            // With no request of its own in flight, only the call's queue can hear it.
            Bus quiet = client("#j-caller@localhost");
            quiet.pollReplies = false;
            AtomicReference<Throwable> quietEnd = new AtomicReference<>();
            Thread q = Thread.ofVirtual().start(() -> quietEnd.set(thrown(() -> quiet.call("#j-slow@localhost", "x", Call.within(Duration.ofSeconds(8))))));
            Thread.sleep(300);
            long q0 = System.currentTimeMillis();
            quiet.close();
            q.join(3000);
            check("close wakes a call that has nothing in flight", quietEnd.get() instanceof BusException && System.currentTimeMillis() - q0 < 1500,
                    name(quietEnd.get()));
            check("and ends a consume in flight", takeEnd.get() instanceof BusException && took < 1500, name(takeEnd.get()));
            client("#j-slow@localhost").consume(Consume.waiting(ms(300)));
            Bus b = client("#j-svc@localhost");
            Record r = b.self();
            r.config();
            r.secret();
            b.close();
            check("a send after close is refused", thrown(() -> b.send("#j-other@localhost", "x")) instanceof BusException);
            check("a closed client answers no cached config", thrown(r::config) instanceof BusException);
            check("nor a cached secret", thrown(r::secret) instanceof BusException);
        });

        section("protocol rule and refusal kind", () -> {
            Path sock = Path.of(DIR, "p-ok.sock");
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                Bus b = Bus.connect(Connect.of(sock.toString(), "x"));
                b.status();
                check("a daemon at the client's protocol is silent", b.warnings().isEmpty(), b.warnings());
                check("the handshake is one GET /identity, once", f.count("GET /identity") == 0 && f.seen.get() == 1, f.lines);
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                f.identity = new Answer(0, 200, "{\"protocol\":5,\"protocol_min\":1}");
                Bus b = Bus.connect(Connect.of(sock.toString(), "x"));
                b.status();
                check("a newer daemon still within protocol_min is silent", b.warnings().isEmpty(), b.warnings());
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                f.identity = new Answer(0, 200, "{\"protocol\":2,\"protocol_min\":2}");
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).status());
                check("a daemon whose protocol_min is above the client is too new",
                        e instanceof BusException.DaemonTooNewException && e.getMessage().equals("daemon too new for this client"), name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                f.identity = null;
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).status());
                check("an identity answer with neither member counts as protocol 0: a warning, not an error", e == null, name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                f.identity = new Answer(0, 200, "{}");
                Bus b = Bus.connect(Connect.of(sock.toString(), "x"));
                b.status();
                check("an identity without the members warns the daemon is too old",
                        b.warnings().size() == 1 && b.warnings().get(0).equals("daemon too old: protocol is older than this client"), b.warnings());
            }
            for (String[] bad : new String[][] {
                    {"{\"protocol\":\"1\",\"protocol_min\":1}", "protocol error: protocol must be a nonnegative integer"},
                    {"{\"protocol\":-1,\"protocol_min\":0}", "protocol error: protocol must be a nonnegative integer"},
                    {"{\"protocol\":1.5,\"protocol_min\":1}", "protocol error: protocol must be a nonnegative integer"},
                    {"{\"protocol\":2,\"protocol_min\":\"x\"}", "protocol error: protocol_min must be a nonnegative integer"},
                    {"{\"protocol\":1,\"protocol_min\":2}", "protocol error: protocol_min exceeds protocol"}}) {
                try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                    f.identity = new Answer(0, 200, bad[0]);
                    Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).status());
                    check("a malformed identity is a protocol error naming the member: " + bad[0],
                            e instanceof BusException.ProtocolErrorException && e.getMessage().equals(bad[1]), name(e));
                }
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 404, "{\"error\":\"no such route\",\"kind\":\"no_route\"}"))) {
                f.identity = new Answer(0, 200, "{\"protocol\":0,\"protocol_min\":0}");
                Bus b = Bus.connect(Connect.of(sock.toString(), "x"));
                Throwable e = thrown(() -> b.list(Optional.empty()));
                check("no_route from an older daemon says too old for the operation",
                        e instanceof BusException.NotFound n && "no_route".equals(n.kind()) && e.getMessage().equals("daemon too old for GET /ls"), name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 404, "{\"error\":\"no such route\",\"kind\":\"no_route\"}"))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).list(Optional.empty()));
                check("no_route from a daemon at the client's protocol keeps the daemon's own words",
                        e instanceof BusException.NotFound n && "no_route".equals(n.kind()) && e.getMessage().equals("no such route"), name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 404, "{\"error\":\"no such record\",\"kind\":\"not_found\"}"))) {
                f.identity = new Answer(0, 200, "{\"protocol\":0,\"protocol_min\":0}");
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).list(Optional.empty()));
                check("not_found is missing data, not a missing route, even from an older daemon",
                        e instanceof BusException.NotFound n && "not_found".equals(n.kind()) && e.getMessage().equals("no such record"), name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 403, "{\"error\":\"not yours\",\"kind\":\"acl\"}"))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).list(Optional.empty()));
                check("a refusal carries the daemon's kind", e instanceof BusException.Forbidden fo && "acl".equals(fo.kind()), name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 500, "{\"error\":\"sqlite: disk I/O\"}"))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).list(Optional.empty()));
                check("a 500 is an internal daemon error, its text not passed on",
                        e instanceof BusException.RefusalException r && r.kind() == null && e.getMessage().equals("internal daemon error"), name(e));
            }
            try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 503, "{\"error\":\"store unavailable\",\"kind\":\"unavailable\"}"))) {
                Throwable e = thrown(() -> Bus.connect(Connect.of(sock.toString(), "x")).list(Optional.empty()));
                check("a 503 with a kind keeps its words and kind",
                        e instanceof BusException.RefusalException r && "unavailable".equals(r.kind()) && e.getMessage().equals("store unavailable"), name(e));
            }
        });
        section("tls", () -> {
            String https = env("AB_TEST_TLS"), pin = env("AB_TEST_PIN");
            Bus pinned = Bus.connect(Connect.of(https, TOKEN).pin(pin).expectName(OWNER));
            check("a pinned https:// daemon answers", pinned.name().equals(OWNER));
            String hex = pin.substring("sha256:".length()).toUpperCase();
            StringBuilder colons = new StringBuilder("SHA256:");
            for (int i = 0; i < hex.length(); i += 2) colons.append(i == 0 ? "" : ":").append(hex, i, i + 2);
            AtomicReference<String> colonName = new AtomicReference<>();
            Throwable ce = thrown(() -> colonName.set(Bus.connect(Connect.of(https, TOKEN).pin(colons.toString())).name()));
            check("the pin ignores case and colons", ce == null && OWNER.equals(colonName.get()), name(ce) + " " + colons);
            // A server holding the right key but shown the wrong pin: it must read no request at all.
            java.security.KeyStore ks = java.security.KeyStore.getInstance("PKCS12");
            try (var in = Files.newInputStream(Path.of(env("AB_TEST_P12")))) { ks.load(in, "test".toCharArray()); }
            javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
            kmf.init(ks, "test".toCharArray());
            javax.net.ssl.SSLContext ctx = javax.net.ssl.SSLContext.getInstance("TLS");
            ctx.init(kmf.getKeyManagers(), null, null);
            AtomicInteger requestBytes = new AtomicInteger();
            try (javax.net.ssl.SSLServerSocket ss = (javax.net.ssl.SSLServerSocket) ctx.getServerSocketFactory().createServerSocket(0, 5, java.net.InetAddress.getLoopbackAddress())) {
                Thread srv = Thread.ofVirtual().start(() -> {
                    while (true) {
                        try (java.net.Socket c = ss.accept()) {
                            byte[] buf = new byte[4096];
                            int n = c.getInputStream().read(buf);
                            if (n > 0) {
                                requestBytes.addAndGet(n);
                                c.getOutputStream().write("HTTP/1.1 200 OK\r\nContent-Length: 13\r\nConnection: close\r\n\r\n{\"you\":\"tls\"}".getBytes(StandardCharsets.US_ASCII));
                            }
                        } catch (Exception e) {
                            if (ss.isClosed()) return;
                        }
                    }
                });
                String at = "https://127.0.0.1:" + ss.getLocalPort();
                Throwable e = thrown(() -> Bus.connect(Connect.of(at, "secret-token").pin("sha256:" + "00".repeat(32))).status());
                check("a wrong pin is refused as Transport", e instanceof TransportException, name(e));
                check("and the server read no request: no token left", requestBytes.get() == 0, requestBytes.get());
                Map<String, Object> st = Bus.connect(Connect.of(at, "secret-token").pin(pin)).status();
                check("the right pin reaches the same server", "tls".equals(st.get("you")) && requestBytes.get() > 0, st);
            }
            check("a blank pin matches nothing", thrown(() -> Bus.connect(Connect.of(https, TOKEN).pin(" "))) instanceof IllegalArgumentException);
            check("a pin is for https:// only", thrown(() -> Bus.connect(Connect.of(TCP, TOKEN).pin(pin))) instanceof IllegalArgumentException);
        });



        // The shared state, identity and refusal vectors (src/testdata/client-conformance): kv, locks, identity and
        // refusals; not bigfile. Fixture names are mapped to this daemon's.
        Function<String, List<?>> vectors = file -> {
            try {
                Path cases = Path.of(env("AB_CLIENT_CONFORMANCE_CASES"), file + ".json");
                List<?> all = (List<?>) Json.parse(Files.readString(cases));
                check("the shared " + file + " vectors are not empty", !all.isEmpty(), cases);
                return all;
            } catch (java.io.IOException e) {
                throw new java.io.UncheckedIOException(e);
            }
        };
        /** The step's value as bytes: repeated text, decoded bytes or the text itself. */
        Function<Map<?, ?>, byte[]> stepBytes = step -> {
            if (step.get("repeat") instanceof Map<?, ?> r) return ((String) r.get("text")).repeat(((Number) r.get("count")).intValue()).getBytes(StandardCharsets.UTF_8);
            if (step.get("value_b64") instanceof String b) return java.util.Base64.getDecoder().decode(b);
            return String.valueOf(or(step, "value", "")).getBytes(StandardCharsets.UTF_8);
        };
        /** Runs one step under its expected refusal: the result when it was meant to succeed, else null after a matched refusal. */
        class Vector {
            Object result;
            boolean ok;

            Vector run(String at, Map<?, ?> step, java.util.concurrent.Callable<Object> call) {
                Object want = step.get("expect") instanceof Map<?, ?> e ? e.get("refused") : null;
                Throwable e = thrown(() -> result = call.call());
                if (want == null) {
                    check(at, e == null, name(e));
                    ok = e == null;
                    return this;
                }
                boolean good = e instanceof BusException;
                if (good && want instanceof Map<?, ?> w) {
                    good = e instanceof BusException.RefusalException r && r.status() == ((Number) w.get("status")).intValue()
                            && (w.get("message_has") == null || e.getMessage().contains((String) w.get("message_has")));
                }
                check(at + ": refused as expected", good, e == null ? "it succeeded" : name(e));
                ok = false;
                return this;
            }
        }
        section("shared kv vectors", () -> {
            for (Object c : vectors.apply("kv")) {
                Map<?, ?> k = (Map<?, ?>) c;
                VectorDaemon daemon = new VectorDaemon();
                String rec = "jobs@fixture";
                KV kv = daemon.as("owner@fixture").record(rec).kv();
                int i = 0;
                for (Object so : (List<?>) k.get("steps")) {
                    Map<?, ?> step = (Map<?, ?>) so;
                    String op = (String) step.get("op"), name = String.valueOf(or(step, "name", "")), kind = String.valueOf(or(step, "kind", "string"));
                    String at = k.get("id") + " step " + i++ + " " + op + " " + name;
                    Map<?, ?> expect = step.get("expect") instanceof Map<?, ?> e ? e : Map.of();
                    KV.How how = KV.How.valueOf(String.valueOf(or(step, "how", "set")).toUpperCase());
                    switch (op) {
                        case "set" -> new Vector().run(at, step, () -> {
                            switch (kind) {
                                case "int" -> kv.set(name, Long.parseLong((String) step.get("value")), how);
                                case "json" -> kv.setJson(name, (String) step.get("value"), how);
                                default -> kv.set(name, stepBytes.apply(step), how);
                            }
                            return null;
                        });
                        case "get" -> {
                            Vector v = new Vector().run(at, step, () -> switch (kind) {
                                case "int" -> kv.getInt(name);
                                case "json" -> kv.getJson(name);
                                default -> kv.getBytes(name);
                            });
                            if (!v.ok) continue;
                            boolean same;
                            if (expect.get("value_b64") != null) same = java.util.Base64.getEncoder().encodeToString((byte[]) v.result).equals(expect.get("value_b64"));
                            else if (expect.get("length") != null) same = ((byte[]) v.result).length == ((Number) expect.get("length")).intValue();
                            else if (expect.containsKey("json")) same = Json.write(Json.parse((String) v.result)).equals(Json.write(expect.get("json")));
                            else if (kind.equals("int")) same = String.valueOf(v.result).equals(expect.get("value"));
                            else same = new String((byte[]) v.result, StandardCharsets.UTF_8).equals(expect.get("value"));
                            check(at + ": reads back", same, v.result instanceof byte[] b ? b.length + " bytes" : v.result);
                        }
                        case "fill" -> {
                            Throwable failed = thrown(() -> {
                                for (int j = 0; j < ((Number) step.get("count")).intValue(); j++) kv.set(step.get("prefix") + "" + j, "v", KV.How.ADD);
                            });
                            check(at + ": " + step.get("count") + " names", failed == null, name(failed));
                        }
                        case "delete" -> {
                            Vector v = new Vector().run(at, step, () -> kv.delete(name, KV.Kind.valueOf(kind.toUpperCase())));
                            if (v.ok) check(at + ": deleted flag", v.result.equals(expect.get("deleted")), v.result);
                        }
                        case "inc" -> {
                            Vector v = new Vector().run(at, step, () -> kv.inc(name, Long.parseLong((String) step.get("by"))));
                            if (v.ok && expect.get("value") != null) check(at + ": counter", String.valueOf(v.result).equals(expect.get("value")), v.result);
                        }
                        case "json" -> {
                            List<KV.Op> ops = new ArrayList<>();
                            for (Object oo : (List<?>) step.get("ops")) {
                                Map<?, ?> o = (Map<?, ?>) oo;
                                String key = (String) o.get("key");
                                Object value = o.get("value");
                                ops.add(switch ((String) o.get("op")) {
                                    case "push" -> KV.Op.push(key, value);
                                    case "unshift" -> KV.Op.unshift(key, value);
                                    case "set" -> KV.Op.set(key, value);
                                    case "inc" -> KV.Op.inc(key, ((Number) value).longValue());
                                    case "unset" -> KV.Op.unset(key);
                                    case "shift" -> KV.Op.shift(key);
                                    case "pop" -> KV.Op.pop(key);
                                    case "add_to_set" -> KV.Op.addToSet(key, value);
                                    case "remove_from_set" -> KV.Op.removeFromSet(key, value);
                                    default -> throw new IllegalStateException("unknown json op " + o.get("op"));
                                });
                            }
                            Vector v = new Vector().run(at, step, () -> kv.json(name, ops.toArray(new KV.Op[0])));
                            if (v.ok && expect.get("changed") != null) {
                                List<Object> changed = new ArrayList<>();
                                for (KV.OpResult r : (List<KV.OpResult>) v.result) changed.add(r.changed());
                                check(at + ": changed flags", changed.equals(expect.get("changed")), changed);
                            }
                        }
                        default -> check(at + ": known op", false, op);
                    }
                }
                daemon.close();
            }
        });

        section("shared lock vectors", () -> {
            for (Object c : vectors.apply("locks")) {
                Map<?, ?> k = (Map<?, ?>) c;
                VectorDaemon daemon = new VectorDaemon();
                String worker = "#worker@fixture";
                Bus owner = daemon.as("owner@fixture");
                Map<String, Bus> buses = Map.of("owner", owner, "worker", daemon.as(worker));
                String rec = "jobs@fixture";
                for (Object setup : k.get("setup") instanceof List<?> l ? l : List.of()) {
                    check("setup " + setup + " is known", "worker-maintains-jobs".equals(setup));
                    owner.record(rec).manage(Map.of("maintainers", List.of(worker)));
                }
                Map<String, Locks.Lease> held = new java.util.LinkedHashMap<>();
                int i = 0;
                for (Object so : (List<?>) k.get("steps")) {
                    Map<?, ?> step = (Map<?, ?>) so;
                    String who = (String) step.get("as"), op = (String) step.get("op"), name = String.valueOf(step.get("name"));
                    String at = k.get("id") + " step " + i++ + " " + who + " " + op + " " + name;
                    Locks locks = buses.get(who).record(rec).locks();
                    Map<?, ?> expect = step.get("expect") instanceof Map<?, ?> e ? e : Map.of();
                    Duration ttl = Duration.ofSeconds(step.get("ttl_s") instanceof Number t ? t.longValue() : 0);
                    switch (op) {
                        case "try" -> { Vector v = new Vector().run(at, step, () -> locks.tryAcquire(name, ttl)); if (v.ok) held.put(who + name, (Locks.Lease) v.result); }
                        case "acquire" -> {
                            Duration wait = Duration.ofSeconds(step.get("wait_s") instanceof Number t ? t.longValue() : 0);
                            Vector v = new Vector().run(at, step, () -> locks.acquire(name, ttl, wait));
                            if (v.ok) held.put(who + name, (Locks.Lease) v.result);
                        }
                        case "extend" -> new Vector().run(at, step, () -> { locks.extend(name, ttl); return null; });
                        case "release" -> new Vector().run(at, step, () -> {
                            Locks.Lease lease = held.remove(who + name);
                            if (lease != null && lease.release()) return null;
                            locks.release(name);
                            return null;
                        });
                        case "force" -> {
                            Vector v = new Vector().run(at, step, () -> locks.forceRelease(name).orElse(null));
                            if (v.ok) check(at + ": displaced holder", java.util.Objects.equals(v.result, expect.get("displaced")), v.result);
                        }
                        case "holders" -> {
                            Vector v = new Vector().run(at, step, locks::holders);
                            if (v.ok) {
                                Map<String, String> got = new java.util.LinkedHashMap<>();
                                ((Map<String, Locks.Hold>) v.result).forEach((key, hold) -> got.put(key, hold.holder()));
                                check(at + ": holders", got.equals(expect.get("holders")), got);
                            }
                        }
                        default -> check(at + ": known op", false, op);
                    }
                }
                // The vector held some on purpose; a release here is housekeeping, not part of the case.
                for (Locks.Lease lease : held.values()) thrown(lease::release);
                daemon.close();
            }
        });

        section("shared identity vectors", () -> {
            Path sock = Path.of(DIR, "j-vec-identity.sock");
            for (Object c : vectors.apply("identity")) {
                Map<?, ?> k = (Map<?, ?>) c;
                String id = (String) k.get("id");
                try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                    f.identity = new Answer(0, 200, (String) k.get("identity"));
                    Map<?, ?> expect = (Map<?, ?>) k.get("expect");
                    Bus[] bus = new Bus[1];
                    Throwable e = thrown(() -> { bus[0] = Bus.connect(Connect.of(sock.toString(), "x")); bus[0].status(); });
                    if (expect.get("fatal") != null) {
                        check(id + ": fatal naming it", e != null && e.getMessage().contains((String) expect.get("fatal")), name(e));
                        check(id + ": the daemon saw only /identity", f.all.size() == 1 && f.all.get(0).startsWith("GET /identity"), f.all);
                    } else {
                        check(id + ": serves", e == null, name(e));
                        check(id + ": /identity was first", !f.all.isEmpty() && f.all.get(0).startsWith("GET /identity"), f.all);
                        List<String> warns = bus[0] == null ? List.of() : bus[0].warnings();
                        if (expect.get("warns") != null) check(id + ": warns", warns.size() == 1 && warns.get(0).contains((String) expect.get("warns")), warns);
                        else check(id + ": silent", warns.isEmpty(), warns);
                    }
                }
            }
        });

        section("shared refusal vectors", () -> {
            Path sock = Path.of(DIR, "j-vec-refusal.sock");
            String current = "{\"protocol\":1,\"protocol_min\":1}";
            for (Object c : vectors.apply("refusals")) {
                Map<?, ?> k = (Map<?, ?>) c;
                String id = (String) k.get("id");
                if (k.get("call") instanceof Map<?, ?> call) {
                    try (Fake f = new Fake(sock, (line, i) -> new Answer(0, 200, "{}"))) {
                        Locks locks = Bus.connect(Connect.of(sock.toString(), "x")).record("jobs@localhost").locks();
                        String name = (String) call.get("name");
                        Duration ttl = Duration.ofSeconds(((Number) call.get("ttl_s")).longValue());
                        Throwable e = thrown(() -> {
                            switch ((String) call.get("op")) {
                                case "acquire" -> locks.acquire(name, ttl, Duration.ofSeconds(((Number) call.get("wait_s")).longValue()));
                                case "try" -> locks.tryAcquire(name, ttl);
                                case "extend" -> locks.extend(name, ttl);
                                default -> throw new IllegalStateException("unknown call op");
                            }
                        });
                        check(id + ": refused by the client", e != null && !(e instanceof BusException.RefusalException), name(e));
                        String not = (String) ((Map<?, ?>) k.get("expect")).get("not_requested");
                        check(id + ": " + not + " never reached the daemon", f.all.stream().noneMatch(l -> l.split(" ")[1].split("\\?")[0].equals(not)), f.all);
                    }
                    continue;
                }
                Map<?, ?> daemon = (Map<?, ?>) k.get("daemon"), want = (Map<?, ?>) k.get("expect");
                String allow = daemon.get("allow") == null ? "" : "Allow: " + daemon.get("allow") + "\r\n";
                try (Fake f = new Fake(sock, (line, i) -> new Answer(0, ((Number) daemon.get("status")).intValue(), (String) daemon.get("body"), allow))) {
                    f.identity = new Answer(0, 200, "current".equals(k.get("identity")) ? current : "{}");
                    Bus b = Bus.connect(Connect.of(sock.toString(), "x"));
                    Throwable e = thrown(() -> b.list(Optional.empty()));
                    check(id + ": status, kind and message", e instanceof BusException.RefusalException r && r.status() == ((Number) want.get("status")).intValue()
                            && java.util.Objects.equals(r.kind() == null ? "" : r.kind(), want.get("kind")) && e.getMessage().equals(want.get("message")), name(e));
                    check(id + ": warns only for an old daemon", b.warnings().isEmpty() == !"old".equals(k.get("identity")), b.warnings());
                }
            }
        });


        // The shared event-log and event-subscription vectors: the daemon Owner, the agent whose log it is, a third
        // party and a source Maintainer act on a fixture daemon with its own #bus reader. A subscription copy is read
        // from the destination queue named by the step; no ack or done is sent for it (it is a notice).
        interface Vectors { void run(String file) throws Exception; }
        Vectors eventVectors = file -> {
            Map<String, String> who = Map.of("owner", "owner@fixture", "agent", "#caller@fixture", "third", "#outsider@fixture", "worker", "#worker@fixture");
            for (Object c : vectors.apply(file)) {
                Map<?, ?> k = (Map<?, ?>) c;
                try (VectorDaemon daemon = new VectorDaemon()) {
                    Map<String, Bus> buses = new java.util.HashMap<>();
                    who.forEach((role, name) -> buses.put(role, daemon.as(name)));
                    Map<Integer, String> answers = new java.util.HashMap<>();
                    String[] subscription = {""};
                    java.util.function.Function<Object, Object> sub = v -> v instanceof String t ? t.replace("$subscription", subscription[0]) : v;
                    int i = 0;
                    for (Object so : (List<?>) k.get("steps")) {
                        Map<?, ?> step = (Map<?, ?>) so;
                        int n = i++;
                        String op = (String) step.get("op"), at = k.get("id") + " step " + n + " " + op;
                        Bus bus = buses.get((String) step.get("as"));
                        if (bus == null) { check(at + ": known identity", false, step.get("as")); continue; }
                        Map<?, ?> expect = step.get("expect") instanceof Map<?, ?> e ? e : Map.of();
                        switch (op) {
                            case "call" -> {
                                String body = (String) sub.apply(step.get("body"));
                                Outcome o = bus.call("#bus", body, Call.within(Duration.ofSeconds(5)));
                                if (!(o instanceof Outcome.Answer a)) { check(at + ": answered", false, o); break; }
                                String answer = a.message().body();
                                answers.put(n, answer);
                                Object decoded = Json.parse(answer);
                                if (body.startsWith("sub add ") && decoded instanceof Map<?, ?> row && row.get("id") instanceof String id) subscription[0] = id;
                                else if (body.startsWith("sub add ") && expect.containsKey("target")) check(at + ": sub add returned an id", false, answer);
                                for (Map.Entry<?, ?> e : expect.entrySet()) {
                                    Object got = decoded;
                                    boolean found = true;
                                    for (String key : ((String) e.getKey()).split("\\.")) {
                                        if (got instanceof Map<?, ?> m && m.containsKey(key)) got = m.get(key);
                                        else if (got instanceof List<?> l && key.matches("\\d+") && Integer.parseInt(key) < l.size()) got = l.get(Integer.parseInt(key));
                                        else { found = false; break; }
                                    }
                                    Object want = sub.apply(e.getValue());
                                    check(at + ": " + e.getKey(), found && java.util.Objects.equals(got, want), "got " + got + " want " + want + "; reply " + answer);
                                }
                                if (step.get("same_answer_as") instanceof Number prior) {
                                    check(at + ": the reply is byte for byte step " + prior + "'s", answer.equals(answers.get(prior.intValue())), answer);
                                }
                            }
                            case "send" -> {
                                Throwable e = thrown(() -> bus.send("#bus", (String) step.get("body")));
                                check(at + ": queued", e == null, name(e));
                            }
                            case "manage" -> {
                                @SuppressWarnings("unchecked") Map<String, Object> changes = (Map<String, Object>) step.get("changes");
                                Throwable e = thrown(() -> bus.record((String) step.get("name")).manage(changes));
                                check(at + ": managed", e == null, name(e));
                            }
                            case "inbox" -> {
                                Consume read = Consume.waiting(Duration.ofMillis(300));
                                if (step.get("inbox") instanceof String q) read = read.inbox(q);
                                Optional<Message> m = bus.consume(read);
                                if (Boolean.TRUE.equals(expect.get("empty"))) {
                                    check(at + ": the inbox is empty", m.isEmpty(), m.map(x -> x.from() + ": " + x.body()).orElse(""));
                                    break;
                                }
                                if (m.isEmpty()) { check(at + ": a copy was waiting", false, "the inbox was empty"); break; }
                                Message x = m.get();
                                Map<String, Object> seen = new java.util.HashMap<>();
                                seen.put("from", x.from()); seen.put("to", x.to());
                                seen.put("topic", x.topic().orElse("")); seen.put("tag", x.tag().orElse(""));
                                seen.put("receipt", x.receipt().map(Object::toString).orElse("")); seen.put("re", x.re().orElse(""));
                                seen.put("wait", x.deadline().isPresent() ? "set" : "");   // this client reads a waiting call as a message with a deadline
                                seen.put("body", Json.parse(x.body()));
                                for (Map.Entry<?, ?> e : expect.entrySet()) {
                                    Object got = seen;
                                    boolean found = true;
                                    for (String key : ((String) e.getKey()).split("\\.")) {
                                        if (got instanceof Map<?, ?> mm && mm.containsKey(key)) got = mm.get(key);
                                        else { found = false; break; }
                                    }
                                    check(at + ": copy " + e.getKey(), found && java.util.Objects.equals(got, e.getValue()), "got " + got + " want " + e.getValue());
                                }
                            }
                            default -> check(at + ": known op", false, op);
                        }
                    }
                }
            }
        };
        section("shared event-log vectors", () -> eventVectors.run("events"));
        section("shared event-subscription vectors", () -> eventVectors.run("event-subscriptions"));
        section("event-log example", () -> {
            try (VectorDaemon daemon = new VectorDaemon()) {
                List<String> lines = EventLogExample.run(daemon.addr, daemon.tokens);
                check("the event-log example prints what it documents", lines.equals(List.of(
                        "written: alert disk full",
                        "owner reads: 1 alert, newest disk full",
                        "owner lists: #caller@fixture",
                        "a third party: forbidden",
                        "the writer reading its own log: forbidden")), lines);
            }
        });


        section("event-subscription example", () -> {
            try (VectorDaemon daemon = new VectorDaemon()) {
                List<String> lines = EventSubscriptionExample.run(daemon.addr, daemon.tokens);
                check("the event-subscription example prints what it documents", lines.equals(List.of(
                        "listed: 1 subscription",
                        "delivered: 1 copy",
                        "copy from #bus: alert disk full",
                        "one copy only: true",
                        "after cancelling, matched: 0")), lines);
            }
        });

        System.out.println("passed " + pass + ", failed " + fail);
        System.exit(fail == 0 ? 0 : 1);
    }
}
