package agentbus;

import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * A record's secret: env-file text, as stored. Each {@link Record#secret()}
 * hands out its own instance holding its own copy, so closing one zeroes that
 * copy and never another thread's or the cache's.
 */
public final class Secret implements AutoCloseable {
    private final byte[] bytes;
    private volatile boolean closed;

    Secret(byte[] bytes) { this.bytes = bytes.clone(); }

    /** A copy of the bytes. */
    public byte[] bytes() {
        if (closed) throw new IllegalStateException("this Secret was closed");
        return bytes.clone();
    }

    /** The env file read as names and values: NAME=value per line, # comments and blank lines skipped, one level of matching quotes taken off. */
    public Map<String, String> env() {
        Map<String, String> out = new LinkedHashMap<>();
        for (String line : new String(bytes(), StandardCharsets.UTF_8).split("\r?\n")) {
            line = line.strip();
            if (line.isEmpty() || line.startsWith("#")) continue;
            if (line.startsWith("export ")) line = line.substring(7).strip();
            int eq = line.indexOf('=');
            if (eq < 0) continue;
            String v = line.substring(eq + 1);
            if (v.length() >= 2 && (v.charAt(0) == '"' || v.charAt(0) == '\'') && v.charAt(v.length() - 1) == v.charAt(0)) {
                v = v.substring(1, v.length() - 1);
            }
            out.put(line.substring(0, eq).strip(), v);
        }
        return Collections.unmodifiableMap(out);
    }

    /** Zeroes this instance's bytes. */
    @Override
    public void close() {
        closed = true;
        Arrays.fill(bytes, (byte) 0);
    }

    @Override
    public String toString() { return "Secret[redacted]"; }
}
