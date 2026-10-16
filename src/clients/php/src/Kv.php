<?php
// A record's key-value store (docs/03-records-kv.md#key-value-store):
// strings, int64s and JSON, each its own namespace, every write atomic.
declare(strict_types=1);

namespace AgentBus;

enum Kind: string
{
    case String = 'string';
    case Int = 'int';
    case Json = 'json';
}

/** How a set treats a name already there: always, only if absent, only if present. */
enum How: string
{
    case Set = 'set';
    case Add = 'add';
    case Replace = 'replace';
}

/** One JSON operation on a top-level key, by the daemon's own names. */
final class Op
{
    private function __construct(private string $op, private string $key, private bool $hasValue = false,
                                 private mixed $value = null) {}

    public static function set(string $key, mixed $value): self { return new self('set', $key, true, $value); }
    public static function unset(string $key): self { return new self('unset', $key); }
    public static function inc(string $key, int $by = 1): self { return new self('inc', $key, true, $by); }
    public static function push(string $key, mixed $value): self { return new self('push', $key, true, $value); }
    public static function unshift(string $key, mixed $value): self { return new self('unshift', $key, true, $value); }
    public static function shift(string $key): self { return new self('shift', $key); }
    public static function pop(string $key): self { return new self('pop', $key); }
    public static function addToSet(string $key, mixed $value): self { return new self('add_to_set', $key, true, $value); }
    public static function removeFromSet(string $key, mixed $value): self { return new self('remove_from_set', $key, true, $value); }

    /** @internal */
    public function toArray(): array
    {
        $a = ['op' => $this->op, 'key' => $this->key];
        if ($this->hasValue) { $a['value'] = $this->value; }
        return $a;
    }
}

/** What one JSON operation did: changed, and the element shift or pop took or the number inc left. */
final class OpResult
{
    public function __construct(public readonly string $op, public readonly string $key,
                                public readonly bool $changed, public readonly mixed $value) {}
}

final class Kv
{
    /** @internal */
    public function __construct(private Http $t, private string $record) {}

    /**
     * The stored value: an int, a string (its exact bytes), or the JSON value
     * with objects as stdClass. An absent name throws NotFound — the same 404
     * an unseen record answers, so null would hide a mistyped record.
     */
    public function get(string $name, Kind $kind = Kind::String): mixed
    {
        [, $out] = $this->t->request('GET', '/kv', ['record' => $this->record, 'kind' => $kind->value, 'name' => $name]);
        $a = Json::decodeValue($out);
        if (isset($a->value_base64)) { return base64_decode($a->value_base64, true); }
        return match ($kind) {
            Kind::Int => self::int($a->value),
            default => $a->value,
        };
    }

    /** Writes a value; $how says whether one already there is replaced, refused, or required. */
    public function set(string $name, mixed $value, Kind $kind = Kind::String, How $how = How::Set): void
    {
        $body = ['record' => $this->record, 'kind' => $kind->value, 'name' => $name, 'how' => $how->value];
        switch ($kind) {
            case Kind::Int:
                if (!is_int($value)) { throw new InvalidArgument('an int value is a PHP int, 64-bit'); }
                $body['value'] = $value;
                break;
            case Kind::String:
                if (!is_string($value)) { throw new InvalidArgument('a string value is a PHP string'); }
                // Bytes that are not UTF-8 travel as base64, so nothing is lost.
                if (mb_check_encoding($value, 'UTF-8')) { $body['value'] = $value; }
                else { $body['value_base64'] = base64_encode($value); }
                break;
            case Kind::Json:
                $body['value'] = $value;
                break;
        }
        $this->t->request('POST', '/kv/set', [], $body, true);
    }

    /** Whether there was a value to delete. */
    public function delete(string $name, Kind $kind = Kind::String): bool
    {
        $a = $this->t->json('POST', '/kv/delete', [], ['record' => $this->record, 'kind' => $kind->value, 'name' => $name], true);
        return (bool) ($a['deleted'] ?? false);
    }

    /** Adds $by to an int, an absent one counting from zero; answers the new value. */
    public function inc(string $name, int $by = 1): int
    {
        [, $out] = $this->t->request('POST', '/kv/inc', [], ['record' => $this->record, 'name' => $name, 'n' => $by], true);
        return self::int(Json::decodeValue($out)->value);
    }

    /**
     * Applies the operations to one JSON object, all or none, at most 100.
     *
     * @return list<OpResult>
     */
    public function json(string $name, Op ...$ops): array
    {
        if (!$ops) { throw new InvalidArgument('at least one operation'); }
        [, $out] = $this->t->request('POST', '/kv/json', [],
            ['record' => $this->record, 'name' => $name, 'ops' => array_map(fn(Op $o) => $o->toArray(), $ops)], true);
        return array_map(fn($r) => new OpResult($r->op, $r->key, (bool) $r->changed, $r->value ?? null),
            Json::decodeValue($out)->results ?? []);
    }

    /**
     * Every name, by kind, with its size and a preview.
     *
     * @return array{string: list<array>, int: list<array>, json: list<array>}
     */
    public function list(): array
    {
        $a = $this->t->json('GET', '/kv/list', ['record' => $this->record]);
        $v = $a['values'] ?? [];
        return ['string' => $v['string'] ?? [], 'int' => $v['int'] ?? [], 'json' => $v['json'] ?? []];
    }

    private static function int(mixed $v): int
    {
        if (is_int($v)) { return $v; }
        throw new InvalidArgument('the daemon answered an int this PHP cannot hold');
    }
}
