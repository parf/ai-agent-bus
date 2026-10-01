import agentbus.Bus;
import agentbus.Json;
import agentbus.Options;
import agentbus.Outcome;
import agentbus.KV;

import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.TreeMap;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Semaphore;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.stream.Stream;

/**
 * The crawler: finds every agent on the bus that says [stats], spreads a
 * directory's files across them sixteen calls at a time, checks each answer's
 * SHA-256 against its own, and reports. A file one worker fails is tried once
 * on another — safe here, since stats only read. Then it sends one message
 * round every [relay] agent and back. Its report is kept in its own record's
 * key-value store.
 *
 *   java Crawler DIR [MAX_FILES]     (AGENT_BUS_* set: the harness runs it as #demo@localhost)
 */
public final class Crawler {
    static final Set<String> SKIP = Set.of(".git", "node_modules", "target", "build", "tmp", "dist", "__pycache__", "vendor", ".cache");

    record Result(String worker, String path, long bytes, long lines, long todos, String sha256, long served, long ms) {}

    public static void main(String[] args) throws Exception {
        Path dir = Path.of(args.length > 0 ? args[0] : ".").toRealPath();
        int max = args.length > 1 ? Integer.parseInt(args[1]) : 1500;
        try (Bus bus = Bus.connect(Options.Connect.env())) {
            List<String> workers = agents(bus, "[stats]"), ring = agents(bus, "[relay]");
            if (workers.isEmpty()) throw new IllegalStateException("no agent on the bus says [stats]");
            List<Path> files;
            try (Stream<Path> s = Files.walk(dir)) {
                files = s.filter(p -> dir.relativize(p).toString().isEmpty() || !skipped(dir.relativize(p)))
                        .filter(Files::isRegularFile).filter(p -> size(p) <= 2 << 20).sorted().limit(max).toList();
            }
            System.out.printf("crawling %d files under %s with %d workers: %s%n", files.size(), dir, workers.size(), String.join(", ", workers));

            Map<String, Result> results = new ConcurrentHashMap<>();
            List<String> mismatches = Collections.synchronizedList(new ArrayList<>()), failures = Collections.synchronizedList(new ArrayList<>());
            Map<String, AtomicInteger> retried = new ConcurrentHashMap<>();
            Semaphore inFlight = new Semaphore(16);
            long t0 = System.nanoTime();
            List<Thread> threads = new ArrayList<>();
            for (int i = 0; i < files.size(); i++) {
                Path f = files.get(i);
                int first = i % workers.size();
                inFlight.acquire();
                threads.add(Thread.ofVirtual().start(() -> {
                    try {
                        for (int attempt = 0; attempt < Math.min(2, workers.size()); attempt++) {
                            String w = workers.get((first + attempt) % workers.size());
                            Optional<Result> r = ask(bus, w, f);
                            if (r.isPresent()) {
                                results.put(f.toString(), r.get());
                                if (!r.get().sha256().equals(sha256(f))) mismatches.add(w + " " + f);
                                if (attempt > 0) retried.computeIfAbsent(w, k -> new AtomicInteger()).incrementAndGet();
                                return;
                            }
                        }
                        failures.add(f.toString());
                    } finally {
                        inFlight.release();
                    }
                }));
            }
            for (Thread t : threads) t.join();
            long ms = (System.nanoTime() - t0) / 1_000_000;
            report(dir, results.values(), workers, mismatches, failures, retried, ms, counters(workers));

            // One message round the ring: each agent adds itself and calls the next.
            if (!ring.isEmpty()) {
                long r0 = System.nanoTime();
                Outcome o = bus.call(ring.get(0), Json.write(Map.of("route", ring.subList(1, ring.size()), "hops", List.of())),
                        Options.Call.within(Duration.ofSeconds(30)).topic("relay"));
                String got = o instanceof Outcome.Answer a ? a.message().body() : o.toString();
                System.out.printf("%nrelay round %d agents in %d ms: %s%n", ring.size(), (System.nanoTime() - r0) / 1_000_000, got);
            }

            Map<String, Object> summary = new LinkedHashMap<>();
            summary.put("dir", dir.toString());
            summary.put("files", results.size());
            summary.put("lines", results.values().stream().mapToLong(Result::lines).sum());
            summary.put("bytes", results.values().stream().mapToLong(Result::bytes).sum());
            summary.put("todos", results.values().stream().mapToLong(Result::todos).sum());
            summary.put("mismatches", mismatches.size());
            summary.put("failures", failures.size());
            summary.put("ms", ms);
            bus.self().kv().setJson("last_report", Json.write(summary), KV.How.SET);
            System.out.printf("%nsaved as last_report on %s (run %d)%n", bus.name(), bus.self().kv().inc("runs", 1));
            if (!mismatches.isEmpty() || !failures.isEmpty()) System.exit(1);
        }
    }

    static boolean skipped(Path rel) {
        for (Path part : rel) if (SKIP.contains(part.toString())) return true;
        return false;
    }

    static long size(Path p) {
        try { return Files.size(p); } catch (Exception e) { return Long.MAX_VALUE; }
    }

    /**
     * The agents whose description carries the marker, by name. Not filtered by
     * "reading": a reader between two short polls reads as idle for a moment,
     * and a worker that is really gone costs one failover, not the crawl.
     */
    static List<String> agents(Bus bus, String marker) {
        return bus.list(Optional.of("agent")).stream()
                .filter(r -> String.valueOf(r.getOrDefault("descr", "")).contains(marker))
                .map(r -> String.valueOf(r.get("name"))).sorted().toList();
    }

    static Optional<Result> ask(Bus bus, String worker, Path f) {
        long t = System.nanoTime();
        try {
            Outcome o = bus.call(worker, Json.write(Map.of("path", f.toString())), Options.Call.within(Duration.ofSeconds(30)).topic("stats"));
            if (!(o instanceof Outcome.Answer a)) return Optional.empty();
            Map<?, ?> m = (Map<?, ?>) Json.parse(a.message().body());
            if (m.containsKey("error")) return Optional.empty();
            return Optional.of(new Result(worker, f.toString(), num(m, "bytes"), num(m, "lines"), num(m, "todos"),
                    String.valueOf(m.get("sha256")), num(m, "served"), (System.nanoTime() - t) / 1_000_000));
        } catch (RuntimeException e) {
            return Optional.empty();
        }
    }

    static long num(Map<?, ?> m, String k) { return m.get(k) instanceof Number n ? n.longValue() : 0; }

    /**
     * Each worker's own "files" counter, read from its record's key-value store.
     * Only a record's Owner, Maintainers and own Agent may read that store, so
     * this asks as the daemon's owner when the harness provides one. Otherwise,
     * or for a worker that keeps no counter, the column says so.
     */
    static Map<String, String> counters(List<String> workers) {
        Map<String, String> out = new LinkedHashMap<>();
        String token = System.getenv().getOrDefault("EXAMPLES_OWNER_TOKEN", "");
        // Live, the harness runs on the account's own socket, where no token is the account itself.
        if (token.isEmpty() && System.getenv("EXAMPLES_LIVE") == null) return out;
        try (Bus owner = Bus.connect(Options.Connect.of(System.getenv("AGENT_BUS_ADDR"), token))) {
            for (String w : workers) {
                try {
                    out.put(w, String.valueOf(owner.record(w).kv().getInt("files")));
                } catch (RuntimeException e) {
                    out.put(w, "none kept");
                }
            }
        }
        return out;
    }

    static String sha256(Path f) {
        try {
            return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(Files.readAllBytes(f)));
        } catch (Exception e) {
            return "";
        }
    }

    static String ext(String path) {
        String name = Path.of(path).getFileName().toString();
        int dot = name.lastIndexOf('.');
        return dot > 0 ? name.substring(dot) : "(none)";
    }

    static void report(Path dir, java.util.Collection<Result> rs, List<String> workers, List<String> mismatches,
                       List<String> failures, Map<String, AtomicInteger> retried, long ms, Map<String, String> counters) {
        long lines = rs.stream().mapToLong(Result::lines).sum(), bytes = rs.stream().mapToLong(Result::bytes).sum(),
             todos = rs.stream().mapToLong(Result::todos).sum();
        System.out.printf("%n%d files, %,d lines, %,d bytes, %d TODOs in %d ms%n", rs.size(), lines, bytes, todos, ms);

        System.out.printf("%n%-28s %7s %8s %9s %s%n", "worker", "files", "avg ms", "retries", "its KV \"files\"");
        for (String w : workers) {
            List<Result> mine = rs.stream().filter(r -> r.worker().equals(w)).toList();
            System.out.printf("%-28s %7d %8.1f %9d %s%n", w, mine.size(), mine.stream().mapToLong(Result::ms).average().orElse(0),
                    retried.getOrDefault(w, new AtomicInteger()).get(), counters.getOrDefault(w, "not readable here"));
        }

        Map<String, long[]> byExt = new TreeMap<>();
        for (Result r : rs) {
            long[] a = byExt.computeIfAbsent(ext(r.path()), k -> new long[2]);
            a[0]++;
            a[1] += r.lines();
        }
        System.out.printf("%n%-10s %7s %10s%n", "extension", "files", "lines");
        byExt.entrySet().stream().sorted(Comparator.comparingLong((Map.Entry<String, long[]> e) -> -e.getValue()[1])).limit(8)
                .forEach(e -> System.out.printf("%-10s %7d %,10d%n", e.getKey(), e.getValue()[0], e.getValue()[1]));

        System.out.println("\nlongest files");
        rs.stream().sorted(Comparator.comparingLong(Result::lines).reversed()).limit(5)
                .forEach(r -> System.out.printf("%,8d  %s%n", r.lines(), dir.relativize(Path.of(r.path()))));
        System.out.println("\nmost TODOs");
        rs.stream().filter(r -> r.todos() > 0).sorted(Comparator.comparingLong(Result::todos).reversed()).limit(5)
                .forEach(r -> System.out.printf("%8d  %s%n", r.todos(), dir.relativize(Path.of(r.path()))));

        System.out.printf("%nevery answer's SHA-256 checked here: %s%n", mismatches.isEmpty() ? "all match" : mismatches.size() + " differ: " + mismatches);
        if (!failures.isEmpty()) System.out.println("no worker could do " + failures.size() + ": " + failures.subList(0, Math.min(5, failures.size())));
    }
}
