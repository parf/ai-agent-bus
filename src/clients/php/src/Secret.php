<?php
// A record's secret: env-file text, as stored. var_export, serialize and an
// (array) cast read private properties regardless of __debugInfo, so the
// bytes are not object state at all: they live in a WeakMap only this class
// reads, keyed by the object, and go when it does.
declare(strict_types=1);

namespace AgentBus;

final class Secret implements \Stringable
{
    private static ?\WeakMap $held = null;

    /** @internal */
    public function __construct(string $bytes)
    {
        self::$held ??= new \WeakMap();
        self::$held[$this] = $bytes;
    }

    public function bytes(): string { return self::$held[$this]; }

    /**
     * The env file read as names and values: one NAME=value per line, # comments
     * and blank lines skipped, one level of matching quotes taken off.
     *
     * @return array<string,string>
     */
    public function env(): array
    {
        $out = [];
        foreach (preg_split('/\r?\n/', $this->bytes()) as $line) {
            $line = trim($line);
            if ($line === '' || $line[0] === '#') { continue; }
            if (str_starts_with($line, 'export ')) { $line = ltrim(substr($line, 7)); }
            $eq = strpos($line, '=');
            if ($eq === false) { continue; }
            $v = substr($line, $eq + 1);
            if (strlen($v) >= 2 && ($v[0] === '"' || $v[0] === "'") && $v[-1] === $v[0]) { $v = substr($v, 1, -1); }
            $out[trim(substr($line, 0, $eq))] = $v;
        }
        return $out;
    }

    public function __toString(): string { return 'Secret[redacted]'; }

    public function __debugInfo(): array { return ['secret' => 'redacted']; }

    public function __serialize(): array { throw new \LogicException('a Secret is not serialized'); }

    public function __unserialize(array $data): void { throw new \LogicException('a Secret is not serialized'); }

    /** A clone could not reach the original's bytes; fetch the secret again instead. */
    public function __clone() { throw new \LogicException('a Secret is not cloned'); }
}
