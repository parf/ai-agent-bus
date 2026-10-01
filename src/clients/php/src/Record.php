<?php
// A record the caller acts ON: another's, or its own through Bus::me(). Its
// store, its locks and its private values live here, so reading somebody
// else's config shows in the code.
declare(strict_types=1);

namespace AgentBus;

final class Record
{
    public readonly Kv $kv;
    public readonly Locks $locks;
    /** The JSON text, cached; null when nothing is cached. */
    private ?string $config = null;
    private ?Secret $secret = null;

    /** @internal */
    public function __construct(private Http $t, public readonly string $name)
    {
        $this->kv = new Kv($t, $name);
        $this->locks = new Locks($t, $name);
    }

    /** The record as the caller may see it. */
    public function lookup(): array
    {
        return $this->t->json('GET', '/lookup', ['name' => $this->name]);
    }

    /**
     * Changes settings: descr, addr, protocol, allow, subs, maintainers, status,
     * personal, owner, ttl, bound, overflow, roles, or add / add_to_set / remove
     * deltas — as the daemon's /manage takes them.
     */
    public function manage(array $changes): array
    {
        return $this->t->json('POST', '/manage', [], ['name' => $this->name] + $changes, true);
    }

    public function unregister(): void
    {
        $this->t->request('POST', '/unregister', [], ['name' => $this->name], true);
    }

    /**
     * The stored configuration, null when none is. Fetched on first use and
     * cached on this object as text; each call decodes afresh, so changing the
     * answer never changes the cache. Objects are stdClass, so {} and [] stay
     * apart. A refusal throws and is not cached.
     */
    public function config(): mixed
    {
        if ($this->config === null) {
            [, $out] = $this->t->request('GET', '/config', ['name' => $this->name]);
            $this->config = trim($out);
        }
        return Json::decodeValue($this->config);
    }

    /**
     * The secret, fetched on first use and cached on this object. An absent
     * secret is a 404 like an unseen record: NotFound either way.
     */
    public function secret(): Secret
    {
        if ($this->secret === null) {
            [, $out] = $this->t->request('GET', '/secret', ['name' => $this->name]);
            $this->secret = new Secret($out);
        }
        return $this->secret;
    }

    /**
     * Forgets both. A cached value outlives a rotated secret or a removed
     * Maintainer until this, or until the object goes: it is a copy, and
     * revocation never reaches it.
     */
    public function refresh(): void
    {
        $this->config = null;
        $this->secret = null;
    }
}
