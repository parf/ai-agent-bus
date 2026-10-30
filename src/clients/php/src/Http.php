<?php
// One curl handle per client: a unix socket or HTTP/HTTPS, the token in
// X-Agent-Bus-Token. Single-threaded, as PHP is.
declare(strict_types=1);

namespace AgentBus;

/** @internal */
final class Http
{
    private \CurlHandle $curl;
    private string $base;
    private ?string $socket = null;
    private bool $closed = false;

    public function __construct(private string $addr, private string $token, private float $timeout)
    {
        if (str_starts_with($addr, 'http://') || str_starts_with($addr, 'https://')) {
            $this->base = rtrim($addr, '/');
        } else {
            $this->base = 'http://localhost';
            $this->socket = $addr;
        }
        $this->curl = curl_init();
    }

    public function __debugInfo(): array
    {
        return ['addr' => $this->addr, 'timeout' => $this->timeout, 'token' => '[redacted]'];
    }

    public function addr(): string { return $this->addr; }

    public function timeout(): float { return $this->timeout; }

    /**
     * One call. Answers [status, body]; a refusal throws. $write says whether a
     * failure after connecting leaves the outcome unknown.
     *
     * @param array<string,string> $query
     */
    public function request(string $method, string $path, array $query = [], mixed $body = null,
                            bool $write = false, ?float $timeout = null, array $headers = []): array
    {
        if ($this->closed) { throw new BusException('this client is closed'); }
        $url = $this->base . $path . ($query ? '?' . http_build_query($query) : '');
        $h = ['X-Agent-Bus-Token: ' . $this->token];
        foreach ($headers as $k => $v) { $h[] = "$k: $v"; }
        $opts = [
            CURLOPT_URL => $url,
            CURLOPT_CUSTOMREQUEST => $method,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_HTTPHEADER => $h,
            CURLOPT_TIMEOUT_MS => (int) ceil(1000 * ($timeout ?? $this->timeout)),
            CURLOPT_CONNECTTIMEOUT_MS => (int) ceil(1000 * min(10.0, $this->timeout)),
            CURLOPT_POSTFIELDS => null,
            CURLOPT_HTTPGET => $method === 'GET',
        ];
        if ($this->socket !== null) { $opts[CURLOPT_UNIX_SOCKET_PATH] = $this->socket; }
        if ($body !== null) {
            $opts[CURLOPT_POSTFIELDS] = Json::encode($body);
            $opts[CURLOPT_HTTPHEADER][] = 'Content-Type: application/json';
        }
        curl_reset($this->curl);
        curl_setopt_array($this->curl, $opts);
        $out = curl_exec($this->curl);
        if ($out === false) {
            $no = curl_errno($this->curl);
            $why = "agent-bus $method $path at {$this->addr}: " . curl_error($this->curl);
            // Refused before anything left: nothing happened.
            // By number: PHP does not define every one of curl's names.
            // 1 unsupported protocol, 6 host not resolved, 7 no connection,
            // 35 TLS handshake, 60 certificate not trusted.
            if (in_array($no, [1, 6, 7, 35, 60], true)) {
                throw new Transport($why, $no);
            }
            if ($write) { throw new UncertainOutcome("delivery outcome unknown, do not resend: $why", $no); }
            if ($no === 28) { throw new Timeout($why, $no); } // the operation timed out
            throw new Transport($why, $no);
        }
        $status = curl_getinfo($this->curl, CURLINFO_RESPONSE_CODE);
        if ($status >= 400) {
            $msg = trim((string) $out);
            $j = json_decode($msg, true);
            if (is_array($j) && isset($j['error']) && is_string($j['error'])) { $msg = $j['error']; }
            throw Refusal::of($status, $msg);
        }
        return [$status, (string) $out];
    }

    /** The answer decoded: arrays for objects, big ints as digit strings. */
    public function json(string $method, string $path, array $query = [], mixed $body = null,
                         bool $write = false, ?float $timeout = null, array $headers = []): mixed
    {
        [$status, $out] = $this->request($method, $path, $query, $body, $write, $timeout, $headers);
        if ($status === 204 || $out === '') { return null; }
        return Json::decodeArray($out);
    }

    /** Every later request, through any Bus or Record sharing this, is refused. */
    public function close(): void
    {
        $this->closed = true;
        curl_reset($this->curl);
    }

    public function closed(): bool { return $this->closed; }
}

/** @internal JSON as the library sends and reads it. */
final class Json
{
    public static function encode(mixed $v): string
    {
        return json_encode($v, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_PRESERVE_ZERO_FRACTION | JSON_THROW_ON_ERROR);
    }

    public static function decodeArray(string $s): mixed
    {
        return json_decode($s, true, 512, JSON_BIGINT_AS_STRING | JSON_THROW_ON_ERROR);
    }

    /** Objects as stdClass, so {} and [] stay apart. */
    public static function decodeValue(string $s): mixed
    {
        return json_decode($s, false, 512, JSON_BIGINT_AS_STRING | JSON_THROW_ON_ERROR);
    }

    /** A duration in seconds, or a Go duration string as the daemon reads it. */
    /** A wait of nothing: zero seconds, or a string such as "0s" or "0ms". */
    public static function noWait(int|float|string $d): bool
    {
        return is_string($d) ? preg_match('/^\s*0+(\.0*)?\s*(ns|us|µs|ms|s|m|h)?\s*$/u', $d) === 1 : $d == 0;
    }

    public static function duration(int|float|string $d): string
    {
        if (is_string($d)) { return $d; }
        if ($d < 0) { throw new InvalidArgument('a duration is not negative'); }
        return ((int) round($d * 1000)) . 'ms';
    }

    public static function seconds(int|float|string $d): float
    {
        if (!is_string($d)) { return (float) $d; }
        if (!preg_match_all('/(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/', $d, $m, PREG_SET_ORDER) ||
            implode('', array_map(fn($x) => $x[0], $m)) !== $d) {
            throw new InvalidArgument("not a duration: $d");
        }
        $unit = ['ns' => 1e-9, 'us' => 1e-6, 'µs' => 1e-6, 'ms' => 1e-3, 's' => 1, 'm' => 60, 'h' => 3600];
        return array_sum(array_map(fn($x) => (float) $x[1] * $unit[$x[2]], $m));
    }
}
