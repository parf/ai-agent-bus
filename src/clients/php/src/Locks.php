<?php
// A record's shared locks (docs/01-identity-and-authority.md#shared-locks).
// A hold belongs to the caller's NAME: a second take under the same name is
// refused at once (409), not queued, and any process acting as that name can
// release it. There is no lease or fencing token on the wire.
declare(strict_types=1);

namespace AgentBus;

final class Locks
{
    /** @internal */
    public function __construct(private Http $t, private string $record) {}

    /**
     * Takes the lock, waiting up to $wait (the daemon caps it at 60 s). A refusal
     * — held by somebody, or already held by this name — throws Busy.
     */
    public function acquire(string $name, int|float|string $ttl = 60, int|float|string $wait = 0): Lease
    {
        $w = Json::seconds($wait);
        $start = microtime(true);
        $this->t->request('POST', '/lock', ['wait' => Json::duration($w)],
            ['record' => $this->record, 'name' => $name, 'ttl' => Json::duration($ttl)], true,
            $this->t->timeout() + $w);
        return new Lease($this, $name, $start + Json::seconds($ttl));
    }

    /** acquire with no wait. A refusal propagates as Busy; no null pretends to tell held from self-held. */
    public function tryAcquire(string $name, int|float|string $ttl = 60): Lease
    {
        $start = microtime(true);
        $this->t->request('POST', '/try-lock', [], ['record' => $this->record, 'name' => $name, 'ttl' => Json::duration($ttl)], true);
        return new Lease($this, $name, $start + Json::seconds($ttl));
    }

    public function extend(string $name, int|float|string $ttl): void
    {
        $this->t->request('POST', '/extend', [], ['record' => $this->record, 'name' => $name, 'ttl' => Json::duration($ttl)], true);
    }

    public function release(string $name): void
    {
        $this->t->request('POST', '/release', [], ['record' => $this->record, 'name' => $name], true);
    }

    /** Releases whoever holds it; audited. Answers the displaced holder, or null when none held it. */
    public function forceRelease(string $name): ?string
    {
        $a = $this->t->json('POST', '/release-force', [], ['record' => $this->record, 'name' => $name], true);
        $h = $a['holder'] ?? '';
        return $h !== '' ? $h : null;
    }

    /** @return array<string, array{holder: string, expires: \DateTimeImmutable}> */
    public function holders(): array
    {
        $a = $this->t->json('GET', '/holders', ['record' => $this->record]);
        $out = [];
        foreach ($a['locks'] ?? [] as $name => $l) {
            $out[$name] = ['holder' => $l['holder'], 'expires' => new \DateTimeImmutable($l['expires'])];
        }
        return $out;
    }
}

/**
 * A hold as this process took it. A convenience, not a fence: a release names
 * only the record, the lock and the caller, so a stale Lease can release a
 * later hold by the same name. It refuses locally once past its own known
 * expiry, which narrows that window and cannot close it.
 */
final class Lease
{
    private float $expires;
    private bool $released = false;

    /**
     * @internal $expires counts from before the request left, so it is never
     * later than the daemon's own.
     */
    public function __construct(private Locks $locks, public readonly string $name, float $expires)
    {
        $this->expires = $expires;
    }

    /** When the hold ends unless extended, as far as this process knows. */
    public function expires(): \DateTimeImmutable
    {
        return \DateTimeImmutable::createFromFormat('U.u', sprintf('%.6F', $this->expires));
    }

    public function extend(int|float|string $ttl): void
    {
        $this->live();
        $start = microtime(true);
        $this->locks->extend($this->name, $ttl);
        $this->expires = $start + Json::seconds($ttl);
    }

    /** Releases the hold. False, and nothing sent, when it is already past its known expiry or released. */
    public function release(): bool
    {
        if ($this->released || microtime(true) >= $this->expires) {
            $this->released = true;
            return false;
        }
        $this->released = true;
        $this->locks->release($this->name);
        return true;
    }

    /**
     * Only says so. A destructor does not run on a fatal error or a kill, and one
     * that ran late could release somebody's later hold, so it never releases.
     */
    public function __destruct()
    {
        if (!$this->released && microtime(true) < $this->expires) {
            error_log("agent-bus: lock {$this->name} left held; it ends by its ttl");
        }
    }

    private function live(): void
    {
        if ($this->released) { throw new LeaseExpired("lock {$this->name} was released"); }
        if (microtime(true) >= $this->expires) { throw new LeaseExpired("lock {$this->name} is past its ttl"); }
    }
}
