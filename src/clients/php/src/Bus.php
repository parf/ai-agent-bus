<?php
// The caller: whoever the credential says (docs/02-access.md#what-a-call-carries).
// A name argument never makes it somebody else; with $as it is only checked.
// Plans/R1.0-Release/done/client-libraries/php.md owns the design.
declare(strict_types=1);

namespace AgentBus;

final class Bus
{
    private ?string $name = null;
    private bool $stop = false;
    private bool $serving = false;

    private function __construct(private Http $t, private ?string $as) {}

    /**
     * A client. Defaults are the CLI's: AGENT_BUS_ADDR, AGENT_BUS_TOKEN and
     * AGENT_BUS_NAME. No request is made here; with $as, the first call asks
     * the daemon who the credential is and throws IdentityMismatch when it is
     * somebody else.
     */
    public static function connect(?string $addr = null, ?string $token = null, ?string $as = null,
                                   float $timeout = 30.0): self
    {
        if (PHP_INT_SIZE < 8) { throw new InvalidArgument('a 64-bit PHP is required: KV ints are int64'); }
        $token ??= getenv('AGENT_BUS_TOKEN') ?: '';
        $addr ??= getenv('AGENT_BUS_ADDR') ?: self::socket($token !== '');
        if (str_starts_with($addr, 'https://') && getenv('AGENT_BUS_TLS_FINGERPRINT')) {
            throw new InvalidArgument('a pinned https:// daemon is not supported by this client yet; use a socket or a CA-trusted certificate');
        }
        $as ??= getenv('AGENT_BUS_NAME') ?: null;
        return new self(new Http($addr, $token, $timeout), $as !== null ? strtolower(trim($as)) : null);
    }

    /** Who the credential is, as the daemon answers it. */
    public function name(): string
    {
        return $this->who(null);
    }

    /** name(), its request bounded by $timeout seconds when given. */
    private function who(?float $timeout): string
    {
        if ($this->name === null) {
            $you = (string) ($this->t->json('GET', '/status', [], null, false, $timeout)['you'] ?? '');
            if ($this->as !== null && $you !== $this->as) {
                throw new IdentityMismatch("the credential is $you, not {$this->as}");
            }
            $this->name = $you;
        }
        return $this->name;
    }

    /** Totals, counters, and who the caller is with what authority. */
    public function status(): array { $this->check(); return $this->t->json('GET', '/status'); }

    /** The node: its release, owner and host. Needs no credential. */
    public function identity(): array { return $this->t->json('GET', '/identity'); }

    public function me(): Record { return new Record($this->t, $this->name()); }

    public function record(string $name): Record { $this->check(); return new Record($this->t, $name); }

    /** The records the caller may see, optionally of one kind. */
    public function list(?string $kind = null): array
    {
        $this->check();
        return $this->t->json('GET', '/ls', $kind !== null ? ['kind' => $kind] : []);
    }

    /** States a record, owned by the caller. With $createOnly, an existing name is refused. */
    public function register(array $record, bool $createOnly = false): Record
    {
        $this->check();
        $r = $this->t->json('POST', '/register', [], $record, true, null, $createOnly ? ['If-None-Match' => '*'] : []);
        return new Record($this->t, (string) $r['name']);
    }

    /** Accepted by the bus; that says nothing about anybody reading it. Answers the message id. */
    public function send(string $to, string $body, SendOptions $o = new SendOptions()): string
    {
        $this->check();
        return $this->post($to, $body, $o);
    }

    /**
     * The next message, or null when none came within the wait. A failure once
     * the request left throws UncertainOutcome: a message may have been taken
     * and lost with the answer. Never retried here.
     */
    public function consume(ConsumeOptions $o = new ConsumeOptions()): ?Message
    {
        $this->check();
        if ($this->serving) { throw new InvalidArgument('consume while serve() runs would take its messages'); }
        return $this->take($o);
    }

    /**
     * Sends and waits for the answer, matched on a fresh tag. An ack is noted;
     * a done or an answer ends it; the deadline ends it as TimedOut, which never
     * means the remote work stopped.
     */
    public function call(string $to, string $body, CallOptions $o = new CallOptions()): Outcome
    {
        // One budget for the whole exchange, from before anything is sent: the
        // identity check, the send and every wait each get what is left of it.
        if ($o->timeout <= 0) { throw new InvalidArgument('a call needs a positive timeout'); }
        $until = microtime(true) + $o->timeout;
        $this->closedCheck();
        if ($this->as !== null) { $this->who(max(0.001, $until - microtime(true))); }
        $tag = 'call-' . bin2hex(random_bytes(8));
        $inbox = $o->replyTo;
        $id = $this->post($to, $body, new SendOptions($o->topic, $tag, $o->ttl, $o->timeout,
            $inbox !== null ? new ReplyTo($inbox, $o->topic, $tag) : null), null, null, max(0.001, $until - microtime(true)));
        $acked = false;
        while (($left = $until - microtime(true)) > 0) {
            try {
                $m = $this->take(new ConsumeOptions($inbox, $o->topic, $tag, min($left, 30.0)), $left);
            } catch (UncertainOutcome $e) {
                // Only the budget running out inside the wait is a timeout;
                // a connection lost sooner is a failure the caller must hear.
                if (microtime(true) >= $until - 0.005) { break; }
                throw $e;
            }
            if ($m === null) { continue; }
            // A receipt is this call's only when it names this call's message.
            if ($m->isReceipt() && $m->re !== $id) { continue; }
            if ($m->receipt === 'ack') { $acked = true; continue; }
            if ($m->receipt === 'done') { return new Done($m); }
            return new Answer($m, $acked);
        }
        return new TimedOut($acked);
    }

    /** Answers a message where it asked: its reply_to, or its sender, with its topic and tag. */
    public function reply(Message $m, string $body): string
    {
        return $this->respond($m, $body, null);
    }

    public function ack(Message $m): void { $this->respond($m, '', 'ack'); }

    public function done(Message $m): void { $this->respond($m, '', 'done'); }

    /** A publication: a send to a 📣 channel, which copies it to its Deliver-To list. */
    public function publish(string $channel, string $body, SendOptions $o = new SendOptions()): string
    {
        return $this->send($channel, $body, $o);
    }

    /** Takes the caller itself off a channel's Deliver-To list. */
    public function unsubscribe(string $channel): void
    {
        $this->check();
        $this->t->request('POST', '/subscribe', [], ['channel' => $channel, 'off' => true], true);
    }

    /**
     * Serves the caller's inbox until stop(), SIGTERM or SIGINT, maxMessages or
     * idle. Each request is acked, handed to $handler, and answered with the
     * string it returns, or done for null. A throwing handler is told to the
     * log and sends nothing more. A request whose caller already gave up is not
     * run. One message at a time: a taken message is lost if the process dies,
     * so nothing is taken ahead.
     *
     * @param callable(Message, Bus): ?string $handler
     */
    public function serve(callable $handler, ServeOptions $o = new ServeOptions()): int
    {
        $this->check();
        $log = $o->log ?? fn(string $s) => error_log($s);
        $this->stop = false;
        $restore = $this->signals();
        $this->serving = true;
        $served = 0;
        $idleSince = microtime(true);
        try {
            while (!$this->stop) {
                if ($o->maxMessages !== null && $served >= $o->maxMessages) { break; }
                if ($o->idle !== null && microtime(true) - $idleSince >= $o->idle) { break; }
                $m = $this->take(new ConsumeOptions(null, $o->topic, null, $o->poll, $o->share));
                if ($m === null) { continue; }
                $idleSince = microtime(true);
                // A stray receipt or a late answer to an earlier call is no request:
                // an answer names the message it answers.
                if ($m->isReceipt() || $m->re !== null) { continue; }
                if ($m->tooLate()) {
                    $log("agent-bus: {$m->id} arrived after its caller gave up; not run");
                    continue;
                }
                $this->ack($m);
                $served++;
                try {
                    $answer = $handler($m, $this);
                } catch (\Throwable $e) {
                    // The type only: a message may carry the request or a private value.
                    $log("agent-bus: handler failed on {$m->id}: " . get_class($e));
                    continue;
                }
                // Closed by the handler itself: nothing can be sent any more.
                if ($this->t->closed()) { break; }
                $answer === null ? $this->done($m) : $this->reply($m, (string) $answer);
            }
        } finally {
            $this->serving = false;
            $restore();
        }
        return $served;
    }

    /** Ends serve() after the current message. Safe from a signal handler. */
    public function stop(): void { $this->stop = true; }

    /**
     * Closes the client: every later call through it, or through a Record it
     * handed out, is refused, and serve() returns after the message in hand.
     */
    public function close(): void { $this->stop = true; $this->t->close(); }

    private function check(): void
    {
        $this->closedCheck();
        if ($this->as !== null) { $this->name(); }
    }

    private function closedCheck(): void
    {
        if ($this->t->closed()) { throw new BusException('this client is closed'); }
    }

    private function post(string $to, string $body, SendOptions $o, ?string $receipt = null, ?string $re = null,
                          ?float $timeout = null): string
    {
        $e = ['to' => $to, 'body' => $body];
        if ($o->topic !== null) { $e['topic'] = $o->topic; }
        if ($o->tag !== null) { $e['tag'] = $o->tag; }
        if ($o->ttl !== null) { $e['ttl'] = Json::duration($o->ttl); }
        if ($o->wait !== null) { $e['wait'] = Json::duration($o->wait); }
        if ($o->replyTo !== null) { $e['reply_to'] = $o->replyTo->toArray(); }
        if ($receipt !== null) { $e['receipt'] = $receipt; }
        if ($re !== null) { $e['re'] = $re; }
        return (string) ($this->t->json('POST', '/send', [], $e, true, $timeout)['message_id'] ?? '');
    }

    private function respond(Message $m, string $body, ?string $receipt): string
    {
        $this->check();
        $r = $m->route();
        return $this->post($r->name, $body, new SendOptions($r->topic, $r->tag), $receipt, $m->id);
    }

    /** One consume; $limit bounds the whole request, the wait included, when given. */
    private function take(ConsumeOptions $o, ?float $limit = null): ?Message
    {
        $q = ['wait' => Json::duration($o->wait)];
        if ($o->inbox !== null) { $q['inbox'] = $o->inbox; }
        if ($o->topic !== null) { $q['topic'] = $o->topic; }
        if ($o->tag !== null) { $q['tag'] = $o->tag; }
        if ($o->share) { $q['share'] = '1'; }
        // The long poll runs past the client's own timeout by the wait itself.
        // Consuming takes the message off the queue, so a failure once the
        // request has left may have lost one: uncertain, as a send is.
        $e = $this->t->json('GET', '/consume', $q, null, true, $limit ?? $this->t->timeout() + Json::seconds($o->wait));
        return is_array($e) ? Message::fromArray($e) : null;
    }

    /** SIGTERM and SIGINT end serve() after the current message; answers how to put things back. */
    private function signals(): \Closure
    {
        if (!function_exists('pcntl_signal')) { return fn() => null; }
        $async = pcntl_async_signals(true);
        $old = [SIGTERM => pcntl_signal_get_handler(SIGTERM), SIGINT => pcntl_signal_get_handler(SIGINT)];
        foreach ($old as $sig => $_) { pcntl_signal($sig, fn() => $this->stop()); }
        return function () use ($old, $async) {
            foreach ($old as $sig => $h) { pcntl_signal($sig, $h); }
            pcntl_async_signals($async);
        };
    }

    /** The CLI's discovery: the account's own socket without a token, the shared one with. */
    private static function socket(bool $hasToken): string
    {
        $dirs = [];
        if ($d = getenv('XDG_RUNTIME_DIR')) { $dirs[] = "$d/agent-bus"; }
        $dirs[] = '/run/agent-bus';
        $account = function_exists('posix_getpwuid') ? (posix_getpwuid(posix_geteuid())['name'] ?? '') : (getenv('USER') ?: '');
        foreach ($dirs as $dir) {
            $p = !$hasToken && $account !== '' ? "$dir/user-$account.sock" : "$dir/bus.sock";
            if (file_exists($p) && filetype($p) === 'socket') { return $p; }
        }
        $d = getenv('XDG_RUNTIME_DIR');
        return $d ? "$d/agent-bus/bus.sock" : sys_get_temp_dir() . '/agent-bus-' . (function_exists('posix_getuid') ? posix_getuid() : 0) . '.sock';
    }
}
