import agentbus.Bus;
import agentbus.Json;
import agentbus.Message;
import agentbus.Options;
import agentbus.Outcome;
import agentbus.Record;
import agentbus.Server;
import agentbus.Server.Response;

import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * A code-stats worker in Java: the same "stats" and "relay" topics as the PHP
 * one, but four requests at a time on virtual threads. Started by
 * codestats/run.sh with AGENT_BUS_* and STATS_ROOT; serves until SIGTERM.
 */
public final class StatsWorker {
    public static void main(String[] args) throws Exception {
        Path root = Path.of(System.getenv().getOrDefault("STATS_ROOT", System.getenv("EXAMPLES_ROOT"))).toRealPath();
        Bus bus = Bus.connect(Options.Connect.env());
        Record me = bus.self();
        Server s = bus.serve((m, ctx) -> Response.reply(m.topic().orElse("").equals("relay") ? relay(bus, m) : stats(m, root, me)),
                Options.Serve.workers(4).poll(Duration.ofSeconds(1)));
        Runtime.getRuntime().addShutdownHook(new Thread(s::close));
        System.err.println("java stats worker " + bus.name() + " serving files under " + root);
        s.awaitTermination();
    }

    static String stats(Message m, Path root, Record me) throws Exception {
        Map<String, Object> out = new LinkedHashMap<>();
        out.put("worker", "java");
        Object p = ((Map<?, ?>) Json.parse(m.body())).get("path");
        Path real;
        try {
            real = Path.of(String.valueOf(p)).toRealPath();
        } catch (Exception e) {
            real = null;
        }
        // Only files under the root it was given, whatever a request names.
        if (real == null || !real.startsWith(root) || real.equals(root) || !Files.isRegularFile(real)) {
            out.put("error", "not a file under " + root);
            return Json.write(out);
        }
        byte[] data = Files.readAllBytes(real);
        String text = new String(data, java.nio.charset.StandardCharsets.ISO_8859_1);
        long lines = text.chars().filter(c -> c == '\n').count() + (data.length > 0 && data[data.length - 1] != '\n' ? 1 : 0);
        int todos = 0;
        for (int i = text.indexOf("TODO"); i >= 0; i = text.indexOf("TODO", i + 4)) todos++;
        out.put("path", real.toString());
        out.put("bytes", data.length);
        out.put("lines", lines);
        out.put("todos", todos);
        out.put("sha256", HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(data)));
        out.put("served", me.kv().inc("files", 1));
        return Json.write(out);
    }

    /** Adds "java" to the hops and passes the rest of the ring on, answering with what came back. */
    @SuppressWarnings("unchecked")
    static String relay(Bus bus, Message m) {
        Map<String, Object> in = (Map<String, Object>) Json.parse(m.body());
        List<Object> hops = new ArrayList<>((List<Object>) in.getOrDefault("hops", List.of()));
        hops.add("java");
        List<Object> route = new ArrayList<>((List<Object>) in.getOrDefault("route", List.of()));
        if (route.isEmpty()) return Json.write(Map.of("hops", hops));
        String next = String.valueOf(route.remove(0));
        Outcome o = bus.call(next, Json.write(Map.of("route", route, "hops", hops)), Options.Call.within(Duration.ofSeconds(20)).topic("relay"));
        return switch (o) {
            case Outcome.Answer a -> a.message().body();
            case Outcome.Done d -> Json.write(Map.of("hops", hops, "error", next + " finished without an answer"));
            case Outcome.TimedOut t -> Json.write(Map.of("hops", hops, "error", next + " did not answer in time"));
        };
    }
}
