<?php
// Options for the calls that take several. Each field is an existing
// envelope or query field; nothing here is new on the wire.
declare(strict_types=1);

namespace AgentBus;

final class SendOptions
{
    /**
     * @param int|float|string|null $ttl  how long the message is worth delivering
     * @param int|float|string|null $wait how long the sender will wait for an answer; the daemon stamps the deadline
     * @param ?string               $ask  'help', 'help-config', 'help-secret', 'help-json' or 'mcp': asks the receiver about itself instead of giving it work; the body must be empty
     */
    public function __construct(
        public readonly ?string $topic = null,
        public readonly ?string $tag = null,
        public readonly int|float|string|null $ttl = null,
        public readonly int|float|string|null $wait = null,
        public readonly ?ReplyTo $replyTo = null,
        public readonly ?string $ask = null,
    ) {}
}

final class ConsumeOptions
{
    /**
     * @param int|float|string $wait  how long to wait for a message; the daemon caps it at 60 s
     * @param bool             $share this reader is one of a pool reading the same inbox
     */
    public function __construct(
        public readonly ?string $inbox = null,
        public readonly ?string $topic = null,
        public readonly ?string $tag = null,
        public readonly int|float|string $wait = 30,
        public readonly bool $share = false,
    ) {}
}

final class CallOptions
{
    /**
     * @param int|float  $timeout seconds the caller waits; also sent as the message's wait
     * @param ?string    $replyTo an inbox only this caller reads and may consume, for when its own is shared
     * @param ?string    $ask     'help', 'help-config', 'help-secret', 'help-json' or 'mcp', as for SendOptions
     */
    public function __construct(
        public readonly int|float $timeout = 30,
        public readonly string $topic = 'call',
        public readonly int|float|string|null $ttl = null,
        public readonly ?string $replyTo = null,
        public readonly ?string $ask = null,
    ) {}
}

final class ServeOptions
{
    /**
     * @param ?string   $topic       serve only this topic
     * @param ?int      $maxMessages return after this many, so a supervisor can recycle the worker
     * @param int|float|null $idle   return after this long with nothing to do
     * @param bool      $share       one of a pool of processes serving the same name
     * @param int|float $poll        each wait's length: a stop signal is seen at the latest after one
     * @param ?\Closure $log         where a handler failure is told; error_log by default
     */
    public function __construct(
        public readonly ?string $topic = null,
        public readonly ?int $maxMessages = null,
        public readonly int|float|null $idle = null,
        public readonly bool $share = false,
        public readonly int|float $poll = 5,
        public readonly ?\Closure $log = null,
    ) {}
}
