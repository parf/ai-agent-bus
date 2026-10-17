<?php
// A received envelope, every field kept (docs/04-messaging.md#message-fields),
// and what a call can end in.
declare(strict_types=1);

namespace AgentBus;

final class ReplyTo
{
    public function __construct(public readonly string $name, public readonly ?string $topic = null,
                                public readonly ?string $tag = null) {}

    /** @return array<string,string> */
    public function toArray(): array
    {
        return array_filter(['name' => $this->name, 'topic' => $this->topic, 'tag' => $this->tag],
            fn($v) => $v !== null && $v !== '');
    }
}

final class Message
{
    /** @param list<string> $roles */
    public function __construct(
        public readonly string $id,
        public readonly string $from,
        public readonly string $to,
        public readonly string $body,
        public readonly ?string $topic,
        public readonly ?string $tag,
        public readonly \DateTimeImmutable $at,
        public readonly ?string $receipt,
        public readonly ?string $re,
        public readonly ?string $ask,
        public readonly ?ReplyTo $replyTo,
        public readonly array $roles,
        public readonly ?string $originalTo,
        public readonly int $forwards,
        public readonly ?\DateTimeImmutable $expires,
        public readonly ?\DateTimeImmutable $deadline,
    ) {}

    /** @param array<string,mixed> $e */
    public static function fromArray(array $e): self
    {
        $rt = $e['reply_to'] ?? null;
        return new self(
            (string) ($e['message_id'] ?? ''),
            (string) ($e['from'] ?? ''),
            (string) ($e['to'] ?? ''),
            (string) ($e['body'] ?? ''),
            self::text($e['topic'] ?? null),
            self::text($e['tag'] ?? null),
            self::time($e['at'] ?? null) ?? new \DateTimeImmutable('@0'),
            self::text($e['receipt'] ?? null),
            self::text($e['re'] ?? null),
            self::text($e['ask'] ?? null),
            is_array($rt) ? new ReplyTo((string) $rt['name'], self::text($rt['topic'] ?? null), self::text($rt['tag'] ?? null)) : null,
            array_values(array_map('strval', $e['roles'] ?? [])),
            self::text($e['original_to'] ?? null),
            (int) ($e['forwards'] ?? 0),
            self::time($e['expires'] ?? null),
            self::time($e['deadline'] ?? null),
        );
    }

    /** Where an answer to this goes: the sender, unless it named somewhere else. */
    public function route(): ReplyTo
    {
        return $this->replyTo ?? new ReplyTo($this->from, $this->topic, $this->tag);
    }

    public function isReceipt(): bool { return $this->receipt !== null; }

    /** Its caller has stopped waiting: working on it now helps nobody. */
    public function tooLate(?\DateTimeImmutable $now = null): bool
    {
        return $this->deadline !== null && ($now ?? new \DateTimeImmutable()) > $this->deadline;
    }

    private static function text(mixed $v): ?string
    {
        return is_string($v) && $v !== '' ? $v : null;
    }

    private static function time(mixed $v): ?\DateTimeImmutable
    {
        // Go writes an unset moment as the zero time.
        if (!is_string($v) || $v === '' || str_starts_with($v, '0001-01-01')) { return null; }
        return new \DateTimeImmutable($v);
    }
}

/** What a call ended in: an answer, a done receipt, or the caller's deadline. */
abstract class Outcome {}

final class Answer extends Outcome
{
    public function __construct(public readonly Message $message, public readonly bool $acked) {}
}

final class Done extends Outcome
{
    public function __construct(public readonly Message $receipt) {}
}

/** Nothing final came before the deadline. The remote work may still run. */
final class TimedOut extends Outcome
{
    public function __construct(public readonly bool $acked) {}
}
