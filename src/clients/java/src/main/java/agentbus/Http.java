package agentbus;

import agentbus.BusException.RefusalException;
import agentbus.BusException.TimeoutException;
import agentbus.BusException.TransportException;
import agentbus.BusException.UncertainOutcomeException;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.StandardProtocolFamily;
import java.net.URI;
import java.net.URLEncoder;
import java.net.UnixDomainSocketAddress;
import java.nio.channels.AsynchronousCloseException;
import java.nio.channels.Channels;
import java.nio.channels.ClosedByInterruptException;
import java.nio.channels.SocketChannel;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.StringJoiner;
import java.util.concurrent.CancellationException;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import javax.net.ssl.SNIHostName;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLParameters;
import javax.net.ssl.SSLSocket;

/**
 * One request per call, each on its own connection, each written and read here:
 * HTTP/1.1 over a unix socket, plain TCP, or TLS on that TCP connection. No
 * library sits in between, because a client that retries a request it thinks
 * idempotent would repeat a consume — which takes a message off a queue — after
 * a connection lost mid-answer. Nothing here is ever sent twice.
 *
 * <p>Each connection is a blocking {@link SocketChannel}, so a thread's interrupt
 * ends it and {@link #close()} can end every one in flight. A long poll never
 * holds the connection another call needs.
 */
final class Http {
    record Response(int status, byte[] body) {
        String text() { return new String(body, StandardCharsets.UTF_8); }
    }

    private static final ScheduledExecutorService TIMER = Executors.newSingleThreadScheduledExecutor(r -> {
        Thread t = new Thread(r, "agent-bus-timeouts");
        t.setDaemon(true);
        return t;
    });

    private final String addr;
    private final String token;
    private final Duration timeout;
    private final URI tcp;
    private final SSLContext tls;
    private final boolean pinned;
    private volatile boolean closed;
    /** The requests in flight, so close() can end them. */
    private final Set<SocketChannel> inFlight = ConcurrentHashMap.newKeySet();
    /** Requests sent, for the tests' counting. */
    final AtomicLong sent = new AtomicLong();

    /** {@code pin}, for an https:// daemon, is its certificate's SHA-256: checked in the handshake, before any header. */
    Http(String addr, String token, Duration timeout, String pin) {
        this.addr = addr;
        this.token = token;
        this.timeout = timeout;
        if (!isTCP(addr)) {
            if (pin != null) throw new IllegalArgumentException("a certificate pin is for an https:// daemon");
            tcp = null;
            tls = null;
            pinned = false;
            return;
        }
        tcp = URI.create(addr);
        boolean https = addr.startsWith("https://");
        if (pin != null && !https) throw new IllegalArgumentException("a certificate pin is for an https:// daemon");
        pinned = pin != null;
        try {
            tls = !https ? null : pinned ? Pin.context(pin) : SSLContext.getDefault();
        } catch (java.security.NoSuchAlgorithmException e) {
            throw new IllegalStateException(e);
        }
    }

    static boolean isTCP(String addr) { return addr.startsWith("http://") || addr.startsWith("https://"); }

    String addr() { return addr; }

    Duration timeout() { return timeout; }

    /** Every later request is refused, and every one in flight ended, through any Bus or Record sharing this. */
    void close() {
        closed = true;
        for (SocketChannel ch : inFlight) {
            try { ch.close(); } catch (IOException ignored) { }
        }
    }

    boolean closed() { return closed; }

    void open() {
        if (closed) throw new BusException("this client is closed");
    }

    static String query(Map<String, String> q) {
        if (q == null || q.isEmpty()) return "";
        StringJoiner j = new StringJoiner("&", "?", "");
        q.forEach((k, v) -> j.add(URLEncoder.encode(k, StandardCharsets.UTF_8) + "=" + URLEncoder.encode(v, StandardCharsets.UTF_8)));
        return j.toString();
    }

    /**
     * One call; a refusal throws. {@code write} says whether a failure after the
     * request left makes the outcome unknown. An interrupt cancels: it throws
     * {@link CancellationException} and keeps the thread's interrupt flag.
     */
    Response request(String method, String path, Map<String, String> q, Object body, boolean write,
                     Duration wait, Map<String, String> headers) {
        return within(method, path, q, body, write, wait == null ? timeout : timeout.plus(wait), headers);
    }

    /** One call whose whole life, connection to last byte, is bounded by {@code limit}. */
    Response within(String method, String path, Map<String, String> q, Object body, boolean write,
                    Duration limit, Map<String, String> headers) {
        open();
        if (limit.isNegative() || limit.isZero()) limit = Duration.ofMillis(1);
        String target = path + query(q);
        byte[] payload = body == null ? null : Json.write(body).getBytes(StandardCharsets.UTF_8);
        sent.incrementAndGet();
        Response r = exchange(method, target, payload, write, limit, headers);
        if (r.status >= 400) {
            String msg = r.text().trim();
            try {
                Object e = Json.object(msg).get("error");
                if (e instanceof String s) msg = s;
            } catch (RuntimeException ignored) {
                // not the daemon's JSON: the text as it came
            }
            throw RefusalException.of(r.status, msg);
        }
        return r;
    }

    /** The answer decoded, or null for 204 and an empty body. */
    Object json(String method, String path, Map<String, String> q, Object body, boolean write) {
        return json(method, path, q, body, write, timeout);
    }

    Object json(String method, String path, Map<String, String> q, Object body, boolean write, Duration limit) {
        Response r = within(method, path, q, body, write, limit, Map.of());
        return r.status == 204 || r.body.length == 0 ? null : Json.parse(r.text());
    }

    private String what(String method, String target) {
        return "agent-bus " + method + " " + target.split("\\?")[0] + " at " + addr;
    }

    private Response exchange(String method, String target, byte[] payload, boolean write, Duration limit,
                              Map<String, String> headers) {
        int port = tcp == null ? 0 : tcp.getPort() > 0 ? tcp.getPort() : tls != null ? 443 : 80;
        String host = tcp == null ? "localhost" : tcp.getHost() + ":" + port;
        StringBuilder h = new StringBuilder();
        h.append(method).append(' ').append(target).append(" HTTP/1.1\r\n")
         .append("Host: ").append(host).append("\r\n")
         .append("X-Agent-Bus-Token: ").append(token).append("\r\n")
         .append("Connection: close\r\n");
        headers.forEach((k, v) -> h.append(k).append(": ").append(v).append("\r\n"));
        if (payload != null) h.append("Content-Type: application/json\r\nContent-Length: ").append(payload.length).append("\r\n");
        else if (!method.equals("GET")) h.append("Content-Length: 0\r\n");
        h.append("\r\n");
        SocketChannel ch;
        try {
            // The default family for TCP, which takes IPv4 and IPv6 alike.
            ch = tcp == null ? SocketChannel.open(StandardProtocolFamily.UNIX) : SocketChannel.open();
        } catch (IOException e) {
            throw new TransportException(what(method, target) + ": " + e, e);
        }
        inFlight.add(ch);
        AtomicBoolean expired = new AtomicBoolean();
        ScheduledFuture<?> alarm = TIMER.schedule(() -> {
            expired.set(true);
            try { ch.close(); } catch (IOException ignored) { }
        }, limit.toMillis(), TimeUnit.MILLISECONDS);
        boolean left = false;
        SSLSocket ssl = null;
        try (ch) {
            if (closed) throw new BusException("this client is closed");
            InputStream in;
            OutputStream out;
            // Connecting, and a TLS handshake whose certificate is refused, send
            // nothing of the request: a failure here is Transport, never uncertain.
            try {
                ch.connect(tcp == null ? UnixDomainSocketAddress.of(addr) : new InetSocketAddress(bare(tcp.getHost()), port));
                if (tls != null) {
                    ssl = (SSLSocket) tls.getSocketFactory().createSocket(ch.socket(), bare(tcp.getHost()), port, true);
                    SSLParameters p = ssl.getSSLParameters();
                    // A pin is the identity; without one, the host name is checked.
                    if (!pinned) p.setEndpointIdentificationAlgorithm("HTTPS");
                    if (!tcp.getHost().matches("[0-9.]+|\\[.*\\]|.*:.*")) p.setServerNames(List.of(new SNIHostName(tcp.getHost())));
                    ssl.setSSLParameters(p);
                    ssl.startHandshake();
                    in = ssl.getInputStream();
                    out = ssl.getOutputStream();
                } else {
                    in = Channels.newInputStream(ch);
                    out = Channels.newOutputStream(ch);
                }
            } catch (ClosedByInterruptException e) {
                throw e;
            } catch (IOException e) {
                if (Thread.currentThread().isInterrupted()) throw new CancellationException(what(method, target) + ": interrupted");
                if (closed) throw new BusException("this client was closed during " + what(method, target));
                throw new TransportException(what(method, target) + ": " + (expired.get() ? "no connection within " + limit : e.toString()), e);
            }
            left = true;
            out.write(h.toString().getBytes(StandardCharsets.UTF_8));
            if (payload != null) out.write(payload);
            out.flush();
            ByteArrayOutputStream raw = new ByteArrayOutputStream();
            in.transferTo(raw);
            return parse(raw.toByteArray());
        } catch (ClosedByInterruptException e) {
            Thread.currentThread().interrupt();
            throw new CancellationException(what(method, target) + ": interrupted");
        } catch (IOException | RuntimeException e) {
            if (e instanceof BusException be) throw be;
            if (e instanceof CancellationException ce) throw ce;
            if (Thread.currentThread().isInterrupted()) throw new CancellationException(what(method, target) + ": interrupted");
            if (closed) throw new BusException("this client was closed during " + what(method, target));
            if (write && left) throw new UncertainOutcomeException("delivery outcome unknown, do not resend: " + what(method, target) + ": "
                    + (expired.get() ? "timed out after " + limit : e.toString()), e);
            if (expired.get() || e instanceof AsynchronousCloseException) throw new TimeoutException(what(method, target) + ": timed out after " + limit, e);
            throw new TransportException(what(method, target) + ": " + e, e);
        } finally {
            alarm.cancel(false);
            inFlight.remove(ch);
            if (ssl != null) {
                try { ssl.close(); } catch (IOException ignored) { }
            }
        }
    }

    /** An IPv6 literal without the brackets a URL writes around it. */
    private static String bare(String host) {
        return host.startsWith("[") && host.endsWith("]") ? host.substring(1, host.length() - 1) : host;
    }

    /** A whole HTTP/1.1 response, read to the close: status, headers, then a sized or chunked body. */
    static Response parse(byte[] raw) throws IOException {
        int end = indexOf(raw, "\r\n\r\n".getBytes(StandardCharsets.US_ASCII), 0);
        if (end < 0) throw new IOException("the connection closed before an answer");
        String[] head = new String(raw, 0, end, StandardCharsets.ISO_8859_1).split("\r\n");
        String[] status = head[0].split(" ", 3);
        if (status.length < 2 || !status[0].startsWith("HTTP/")) throw new IOException("not an HTTP answer: " + head[0]);
        int code = Integer.parseInt(status[1]);
        boolean chunked = false;
        int length = -1;
        for (int i = 1; i < head.length; i++) {
            int c = head[i].indexOf(':');
            if (c < 0) continue;
            String k = head[i].substring(0, c).trim().toLowerCase(Locale.ROOT), v = head[i].substring(c + 1).trim();
            if (k.equals("transfer-encoding") && v.toLowerCase(Locale.ROOT).contains("chunked")) chunked = true;
            if (k.equals("content-length")) length = Integer.parseInt(v);
        }
        int at = end + 4;
        if (!chunked) {
            int n = length >= 0 ? length : raw.length - at;
            if (at + n > raw.length) throw new IOException("the answer was cut short");
            byte[] body = new byte[n];
            System.arraycopy(raw, at, body, 0, n);
            return new Response(code, body);
        }
        ByteArrayOutputStream body = new ByteArrayOutputStream();
        while (true) {
            int nl = indexOf(raw, "\r\n".getBytes(StandardCharsets.US_ASCII), at);
            if (nl < 0) throw new IOException("the answer was cut short");
            int size = Integer.parseInt(new String(raw, at, nl - at, StandardCharsets.US_ASCII).split(";")[0].trim(), 16);
            at = nl + 2;
            if (size == 0) return new Response(code, body.toByteArray());
            if (at + size > raw.length) throw new IOException("the answer was cut short");
            body.write(raw, at, size);
            at += size + 2;
        }
    }

    private static int indexOf(byte[] b, byte[] p, int from) {
        outer:
        for (int i = from; i + p.length <= b.length; i++) {
            for (int j = 0; j < p.length; j++) if (b[i + j] != p[j]) continue outer;
            return i;
        }
        return -1;
    }
}
