package agentbus;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * A record's crons and one-shot sends, kept and fired by the daemon
 * (docs/09-daemon-api.md#schedules). Answers are the daemon's parsed JSON; the
 * daemon judges every field and its refusal is a {@link BusException}.
 */
public final class Schedules {
    private final Http t;
    private final String record;

    Schedules(Http t, String record) {
        this.t = t;
        this.record = record;
    }

    /** What an add or an edit takes: a target and body, then one of expr, in or at. */
    public static final class Spec {
        private final Map<String, Object> fields = new LinkedHashMap<>();

        private Spec(String target, String body) {
            fields.put("target", target);
            fields.put("body", body);
        }

        /** The message each firing sends, and who receives it. */
        public static Spec to(String target, String body) { return new Spec(target, body); }

        /** Five cron fields, or six with a leading second. */
        public Spec expr(String v) { fields.put("expr", v); return this; }

        /** A delay such as 90s, 30m, 2h or 3d: fires once. */
        public Spec in(String v) { fields.put("in", v); return this; }

        /** HH:MM in UTC or an RFC 3339 instant: fires once. */
        public Spec at(String v) { fields.put("at", v); return this; }

        public Spec until(String v) { fields.put("until", v); return this; }
        public Spec maxRuns(int v) { fields.put("max_runs", v); return this; }
        public Spec maxLate(String v) { fields.put("max_late", v); return this; }
        public Spec disabled(boolean v) { fields.put("disabled", v); return this; }

        Map<String, Object> with(String key, String value) {
            Map<String, Object> b = new LinkedHashMap<>();
            b.put(key, value);
            b.putAll(fields);
            return b;
        }
    }

    /** Keeps a new schedule: {@code {schedule, warning}}, warning set near a cap. */
    public Map<String, Object> add(Spec spec) { return object(t.json("POST", "/schedule/add", null, spec.with("record", record), true)); }

    /** This record's schedules. */
    public List<Map<String, Object>> list() { return rows(t.json("GET", "/schedule/list", Map.of("record", record), null, false)); }

    /** One schedule with its outcomes, newest first. */
    public Map<String, Object> get(String id) { return object(t.json("GET", "/schedule", Map.of("id", id), null, false)); }

    /** Replaces a schedule's time, target, body and limits; the caller becomes its creator. */
    public Map<String, Object> edit(String id, Spec spec) { return change("/schedule/edit", spec.with("id", id)); }

    /** Lets a schedule fire again; the caller becomes its creator. */
    public Map<String, Object> enable(String id) { return change("/schedule/enable", Map.of("id", id)); }

    /** Stops a schedule firing and keeps it. */
    public Map<String, Object> disable(String id) { return change("/schedule/disable", Map.of("id", id)); }

    /** One firing now, answering its outcome; a lost answer is uncertain and never retried. */
    public Map<String, Object> run(String id) { return object(object(t.json("POST", "/schedule/run", null, Map.of("id", id), true)).get("outcome")); }

    /** Removes a schedule; one that is not there is a 404 refusal. */
    public boolean delete(String id) {
        return Boolean.TRUE.equals(object(t.json("POST", "/schedule/delete", null, Map.of("id", id), true)).get("deleted"));
    }

    private Map<String, Object> change(String path, Map<String, Object> body) { return object(object(t.json("POST", path, null, body, true)).get("schedule")); }

    @SuppressWarnings("unchecked")
    private static Map<String, Object> object(Object v) { return (Map<String, Object>) v; }

    @SuppressWarnings("unchecked")
    static List<Map<String, Object>> rows(Object v) { return (List<Map<String, Object>>) object(v).get("schedules"); }
}
