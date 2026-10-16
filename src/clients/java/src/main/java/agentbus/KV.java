package agentbus;

import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * A record's key-value store (docs/03-records-kv.md#key-value-store):
 * strings, int64s and JSON, each kind its own namespace, every write atomic.
 * A read of an absent name throws {@link BusException.NotFound} — the same 404
 * an unseen record answers, so an empty answer would hide a mistyped record.
 */
public final class KV {
    public enum Kind { STRING, INT, JSON;
        String wire() { return name().toLowerCase(); }
    }

    /** How a set treats a name already there: always, only if absent, only if present. */
    public enum How { SET, ADD, REPLACE;
        String wire() { return name().toLowerCase(); }
    }

    /** One JSON operation on a top-level key, by the daemon's own names. {@code value} is any JSON-able value, or {@link Json.Raw}. */
    public record Op(String op, String key, boolean hasValue, Object value) {
        public static Op set(String key, Object v) { return new Op("set", key, true, v); }
        public static Op unset(String key) { return new Op("unset", key, false, null); }
        public static Op inc(String key, long by) { return new Op("inc", key, true, by); }
        public static Op push(String key, Object v) { return new Op("push", key, true, v); }
        public static Op unshift(String key, Object v) { return new Op("unshift", key, true, v); }
        public static Op shift(String key) { return new Op("shift", key, false, null); }
        public static Op pop(String key) { return new Op("pop", key, false, null); }
        public static Op addToSet(String key, Object v) { return new Op("add_to_set", key, true, v); }
        public static Op removeFromSet(String key, Object v) { return new Op("remove_from_set", key, true, v); }

        Map<String, Object> toJson() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("op", op);
            m.put("key", key);
            if (hasValue) m.put("value", value);
            return m;
        }
    }

    /** What one operation did: changed, and the element shift or pop took or the number inc left, as JSON text. */
    public record OpResult(String op, String key, boolean changed, String value) {}

    /** One listed name: its size in bytes and a preview, an int's being the value itself. */
    public record Entry(String name, long size, Object preview) {}

    private final Http t;
    private final String record;

    KV(Http t, String record) {
        this.t = t;
        this.record = record;
    }

    private Map<String, Object> get(String name, Kind kind) {
        Object a = t.json("GET", "/kv", Map.of("record", record, "kind", kind.wire(), "name", name), null, false);
        @SuppressWarnings("unchecked") Map<String, Object> m = (Map<String, Object>) a;
        return m;
    }

    public long getInt(String name) {
        Object v = get(name, Kind.INT).get("value");
        if (v instanceof Long l) return l;
        throw new IllegalStateException("the daemon answered an int that is not an int64: " + v);
    }

    /** A string's bytes exactly, whether or not they are UTF-8. */
    public byte[] getBytes(String name) {
        Map<String, Object> a = get(name, Kind.STRING);
        if (a.get("value_base64") instanceof String b) return Base64.getDecoder().decode(b);
        return String.valueOf(a.get("value")).getBytes(StandardCharsets.UTF_8);
    }

    public String getString(String name) {
        return new String(getBytes(name), StandardCharsets.UTF_8);
    }

    /** The JSON value as its text. */
    public String getJson(String name) {
        return Json.write(get(name, Kind.JSON).get("value"));
    }

    /** The JSON value parsed afresh: maps, lists, Long, BigInteger, BigDecimal, unmodifiable. */
    public Object getJsonTree(String name) {
        return get(name, Kind.JSON).get("value");
    }

    public void set(String name, long value, How how) { write(name, Kind.INT, "value", value, how); }

    public void set(String name, String value, How how) { write(name, Kind.STRING, "value", value, how); }

    /** Bytes that are not UTF-8 travel as base64, so nothing is lost. */
    public void set(String name, byte[] value, How how) {
        if (utf8(value)) write(name, Kind.STRING, "value", new String(value, StandardCharsets.UTF_8), how);
        else write(name, Kind.STRING, "value_base64", Base64.getEncoder().encodeToString(value), how);
    }

    /** A JSON value, given as its text. */
    public void setJson(String name, String json, How how) {
        Json.parse(json);
        write(name, Kind.JSON, "value", new Json.Raw(json), how);
    }

    public void set(String name, long value) { set(name, value, How.SET); }

    public void set(String name, String value) { set(name, value, How.SET); }

    private void write(String name, Kind kind, String field, Object value, How how) {
        Map<String, Object> b = new LinkedHashMap<>();
        b.put("record", record);
        b.put("kind", kind.wire());
        b.put("name", name);
        b.put(field, value);
        b.put("how", how.wire());
        t.json("POST", "/kv/set", null, b, true);
    }

    /** Whether there was a value to delete. */
    public boolean delete(String name, Kind kind) {
        Object a = t.json("POST", "/kv/delete", null, Map.of("record", record, "kind", kind.wire(), "name", name), true);
        return a instanceof Map<?, ?> m && Boolean.TRUE.equals(m.get("deleted"));
    }

    /** Adds {@code by} to an int, an absent one counting from zero; answers the new value. */
    public long inc(String name, long by) {
        Object a = t.json("POST", "/kv/inc", null, Map.of("record", record, "name", name, "n", by), true);
        Object v = ((Map<?, ?>) a).get("value");
        if (v instanceof Long l) return l;
        throw new IllegalStateException("not an int64: " + v);
    }

    /** Applies the operations to one JSON object, all or none, at most 100. */
    public List<OpResult> json(String name, Op... ops) {
        if (ops.length == 0) throw new IllegalArgumentException("at least one operation");
        List<Object> list = new ArrayList<>();
        for (Op o : ops) list.add(o.toJson());
        Map<String, Object> b = new LinkedHashMap<>();
        b.put("record", record);
        b.put("name", name);
        b.put("ops", list);
        Object a = t.json("POST", "/kv/json", null, b, true);
        List<OpResult> out = new ArrayList<>();
        if (((Map<?, ?>) a).get("results") instanceof List<?> rs) {
            for (Object r : rs) {
                Map<?, ?> m = (Map<?, ?>) r;
                out.add(new OpResult(String.valueOf(m.get("op")), String.valueOf(m.get("key")),
                        Boolean.TRUE.equals(m.get("changed")), m.containsKey("value") ? Json.write(m.get("value")) : null));
            }
        }
        return List.copyOf(out);
    }

    /** Every name, by kind. */
    public Map<Kind, List<Entry>> list() {
        Object a = t.json("GET", "/kv/list", Map.of("record", record), null, false);
        Map<?, ?> values = (Map<?, ?>) ((Map<?, ?>) a).get("values");
        Map<Kind, List<Entry>> out = new LinkedHashMap<>();
        for (Kind k : Kind.values()) {
            List<Entry> es = new ArrayList<>();
            if (values != null && values.get(k.wire()) instanceof List<?> l) {
                for (Object o : l) {
                    Map<?, ?> m = (Map<?, ?>) o;
                    Object preview = m.containsKey("preview_base64") ? Base64.getDecoder().decode((String) m.get("preview_base64")) : m.get("preview");
                    es.add(new Entry(String.valueOf(m.get("name")), ((Number) m.get("size")).longValue(), preview));
                }
            }
            out.put(k, List.copyOf(es));
        }
        return out;
    }

    private static boolean utf8(byte[] b) {
        try {
            StandardCharsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT)
                    .onUnmappableCharacter(CodingErrorAction.REPORT).decode(ByteBuffer.wrap(b));
            return true;
        } catch (CharacterCodingException e) {
            return false;
        }
    }
}
