<?php
// The PHP client against a real daemon. Run through tests/daemon.sh, which
// starts one and hands over its address and the owner's token:
//   src/clients/php/tests/daemon.sh php src/clients/php/tests/run.php [filter]
declare(strict_types=1);

require __DIR__ . '/../src/autoload.php';

use AgentBus\{Bus, Record, Message, SendOptions, ConsumeOptions, CallOptions, ServeOptions, ReplyTo,
    Answer, Done, TimedOut, Kind, How, Op, Secret, Refusal, NotFound, Forbidden, Busy, Exists, Invalid,
    Unauthenticated, IdentityMismatch, InvalidArgument, UncertainOutcome, Transport, LeaseExpired};

$addr = getenv('AB_TEST_ADDR') ?: exit("run me through tests/daemon.sh\n");
$owner = getenv('AB_TEST_OWNER');
$ownerToken = getenv('AB_TEST_OWNER_TOKEN');
$dir = getenv('AB_TEST_DIR');
$filter = $argv[1] ?? '';
$pass = 0; $fail = 0;

function check(string $what, bool $ok, string $detail = ''): void
{
    global $pass, $fail;
    if ($ok) { $pass++; echo "  ok   $what\n"; }
    else { $fail++; echo "  FAIL $what" . ($detail !== '' ? ": $detail" : '') . "\n"; }
}

/** The exception $f throws, or null. */
function thrown(callable $f): ?Throwable
{
    try { $f(); } catch (Throwable $e) { return $e; }
    return null;
}

function section(string $name, callable $f): void
{
    global $filter;
    if ($filter !== '' && !str_contains($name, $filter)) { return; }
    echo "== $name\n";
    try { $f(); } catch (Throwable $e) { check("$name ran to the end", false, get_class($e) . ': ' . $e->getMessage()); }
}

/** A raw call as the owner, for setting up what the library does not write. */
function owner(string $method, string $path, ?array $body = null): array
{
    global $addr, $ownerToken;
    $c = curl_init('http://localhost' . $path);
    curl_setopt_array($c, [CURLOPT_UNIX_SOCKET_PATH => $addr, CURLOPT_CUSTOMREQUEST => $method,
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HTTPHEADER => ['X-Agent-Bus-Token: ' . $ownerToken, 'Content-Type: application/json'],
        CURLOPT_POSTFIELDS => $body === null ? null : json_encode($body)]);
    $out = curl_exec($c);
    $code = curl_getinfo($c, CURLINFO_RESPONSE_CODE);
    if ($code >= 400) { throw new RuntimeException("$method $path: $code $out"); }
    return json_decode((string) $out, true) ?? [];
}

function tokenFor(string $name): string { return owner('POST', '/token', ['name' => $name])['token']; }

function client(string $name): Bus
{
    global $addr;
    return Bus::connect($addr, tokenFor($name), $name, 10.0);
}

/** Runs $f in a child process; answers its pid. The child never returns. */
function child(callable $f): int
{
    $pid = pcntl_fork();
    if ($pid === 0) {
        try { $f(); } catch (Throwable $e) { fwrite(STDERR, 'child: ' . $e->getMessage() . "\n"); exit(1); }
        exit(0);
    }
    return $pid;
}

function reap(int $pid, float $within = 10.0): ?int
{
    $until = microtime(true) + $within;
    while (microtime(true) < $until) {
        $r = pcntl_waitpid($pid, $status, WNOHANG);
        if ($r === $pid) { return pcntl_wexitstatus($status); }
        usleep(20000);
    }
    posix_kill($pid, SIGKILL);
    pcntl_waitpid($pid, $status);
    return null;
}


/**
 * A scripted daemon on a unix socket, in a child: each connection gets the
 * answer $script returns for its request line — [delay seconds, status, body].
 * Answers how many requests it saw through $count's file.
 */
function fake(string $sock, callable $script, int $max = 200): int
{
    @unlink($sock);
    @unlink("$sock.count");
    $srv = stream_socket_server("unix://$sock");
    return child(function () use ($srv, $script, $max, $sock) {
        pcntl_signal(SIGPIPE, SIG_IGN); // the client may hang up first: that is the point
        for ($i = 0; $i < $max; $i++) {
            $c = @stream_socket_accept($srv, 5);
            if (!$c) { return; }
            $req = '';
            while (!str_contains($req, "\r\n\r\n") && ($chunk = fread($c, 65536)) !== '' && $chunk !== false) { $req .= $chunk; }
            $bodyAt = strpos($req, "\r\n\r\n");
            if ($bodyAt === false) { fclose($c); continue; }
            $headers = substr($req, 0, $bodyAt);
            $length = preg_match('/^Content-Length:\s*(\d+)/mi', $headers, $match) ? (int) $match[1] : 0;
            // A request can arrive in several reads. Do not answer and close
            // the connection while the client is still writing its body.
            while (strlen($req) - $bodyAt - 4 < $length) {
                $chunk = fread($c, 65536);
                if ($chunk === '' || $chunk === false) { fclose($c); continue 2; }
                $req .= $chunk;
            }
            file_put_contents("$sock.count", (string) ($i + 1));
            [$delay, $status, $body] = $script(strtok($req, "\r\n"), $i);
            usleep((int) ($delay * 1e6));
            if ($status === 'hangup') { fclose($c); continue; }
            @fwrite($c, "HTTP/1.1 $status X\r\nContent-Type: application/json\r\nContent-Length: " . strlen($body) . "\r\nConnection: close\r\n\r\n$body");
            fclose($c);
        }
    });
}

function seen(string $sock): int { return (int) @file_get_contents("$sock.count"); }

function unfake(int $pid): void { posix_kill($pid, SIGKILL); pcntl_waitpid($pid, $status); }

// Fixture: a second person, and the agents and channels the sections use.
owner('POST', '/user', ['name' => 'bob@localhost', 'create' => true]);
$me = Bus::connect($addr, $ownerToken, $owner, 10.0);
foreach (['#php-svc@localhost', '#php-slow@localhost', '#php-pool@localhost', '#php-other@localhost'] as $n) {
    $me->register(['kind' => 'agent', 'name' => $n, 'allow' => ['*']]);
}
$me->register(['kind' => 'queue', 'name' => 'php-replies@localhost', 'allow' => [$owner, '#php-svc@localhost']]);
$me->register(['kind' => 'pubsub', 'name' => 'php-news@localhost', 'allow' => ['*'], 'subs' => ['#php-other@localhost']]);

section('identity', function () use ($addr, $ownerToken, $owner) {
    $b = Bus::connect($addr, $ownerToken, $owner);
    check('the caller is who the credential says', $b->name() === $owner, $b->name());
    $wrong = Bus::connect($addr, $ownerToken, 'bob@localhost');
    $e = thrown(fn() => $wrong->send('#php-svc@localhost', 'x'));
    check('a send as an unexpected name is refused before it leaves', $e instanceof IdentityMismatch, $e ? get_class($e) : 'none');
    check('identity() is the node, not the caller', isset($b->identity()['version']) && !isset($b->identity()['you']));
    $e = thrown(fn() => Bus::connect($addr, 'not-a-token')->list());
    check('a bad token is Unauthenticated', $e instanceof Unauthenticated, $e ? get_class($e) . ' ' . $e->getMessage() : 'none');
});

section('send and consume', function () use ($me) {
    $svc = client('#php-svc@localhost');
    $id = $me->send('#php-svc@localhost', 'hello', new SendOptions('greet', 't1', 60));
    $m = $svc->consume(new ConsumeOptions(wait: 2));
    check('the message arrives with its fields', $m !== null && $m->id === $id && $m->body === 'hello'
        && $m->topic === 'greet' && $m->tag === 't1' && $m->expires !== null, var_export($m, true));
    check('an empty inbox answers null within the wait', $svc->consume(new ConsumeOptions(wait: 0)) === null);
    $me->send('#php-svc@localhost', 'one', new SendOptions('a'));
    $me->send('#php-svc@localhost', 'two', new SendOptions('b'));
    $m = $svc->consume(new ConsumeOptions(topic: 'b', wait: 1));
    check('a topic filter takes the matching one, not the first', $m?->body === 'two', (string) $m?->body);
    check('the other is still there', $svc->consume(new ConsumeOptions(wait: 1))?->body === 'one');
    foreach ([0, 0.0, 0.0004, '0s', '0ms'] as $zero) {
        $e = thrown(fn() => $me->send('#php-svc@localhost', 'zero', new SendOptions(topic: 'z', wait: $zero)));
        check('a zero wait (' . var_export($zero, true) . ') is a send without one', $e === null
            && $svc->consume(new ConsumeOptions(wait: 1))?->body === 'zero', $e ? get_class($e) . ': ' . $e->getMessage() : 'none');
    }
    $e = thrown(fn() => $me->send('#php-svc@localhost', 'neg', new SendOptions(topic: 'z', wait: -1)));
    check('a negative wait is still refused, not dropped', $e instanceof InvalidArgument, $e ? get_class($e) : 'none');
    check('and nothing was delivered', $svc->consume(new ConsumeOptions(wait: 0)) === null);
    $e = thrown(fn() => $me->send('nobody@localhost', 'x'));
    check('a send to an unknown name is NotFound', $e instanceof NotFound, $e ? get_class($e) : 'none');
});

section('reply and receipts', function () use ($me, $owner) {
    $svc = client('#php-svc@localhost');
    $me->send('#php-svc@localhost', 'q', new SendOptions('ask', 'r1'));
    $m = $svc->consume(new ConsumeOptions(wait: 2));
    $svc->ack($m);
    $svc->reply($m, 'a');
    $ack = $me->consume(new ConsumeOptions(topic: 'ask', tag: 'r1', wait: 2));
    $ans = $me->consume(new ConsumeOptions(topic: 'ask', tag: 'r1', wait: 2));
    check('the ack comes back to the sender, naming the request', $ack?->receipt === 'ack' && $ack->re === $m->id);
    check('the reply carries the topic and tag', $ans?->body === 'a' && $ans->receipt === null && $ans->tag === 'r1');
    // reply_to sends the answer elsewhere: to a queue the sender reads.
    $me->send('#php-svc@localhost', 'q2', new SendOptions('ask', 'r2', replyTo: new ReplyTo('php-replies@localhost', 'back', 'r2b')));
    $m = $svc->consume(new ConsumeOptions(wait: 2));
    $svc->reply($m, 'there');
    $got = $me->consume(new ConsumeOptions(inbox: 'php-replies@localhost', topic: 'back', tag: 'r2b', wait: 2));
    check('a reply follows reply_to, its topic and tag', $got?->body === 'there');
    check('and not the sender', $me->consume(new ConsumeOptions(topic: 'ask', tag: 'r2', wait: 0)) === null);
});

section('call', function () use ($me, $addr, $dir) {
    $answers = ['echo' => fn(Message $m) => "echo:{$m->body}", 'none' => fn() => null,
                'empty' => fn() => '', 'boom' => function () { throw new RuntimeException('boom-secret'); }];
    ini_set('error_log', "$dir/php-errors.log");
    $srv = child(function () use ($answers) {
        $svc = client('#php-svc@localhost');
        $svc->serve(fn(Message $m) => $answers[$m->topic]($m), new ServeOptions(maxMessages: 4, poll: 1));
    });
    $o = $me->call('#php-svc@localhost', 'hi', new CallOptions(5, 'echo'));
    check('an answer is an Answer, acked first', $o instanceof Answer && $o->message->body === 'echo:hi' && $o->acked,
        get_class($o) . ' ' . json_encode($o));
    $o = $me->call('#php-svc@localhost', 'x', new CallOptions(5, 'none'));
    check('a handler returning null is Done', $o instanceof Done, get_class($o));
    $o = $me->call('#php-svc@localhost', 'x', new CallOptions(5, 'empty'));
    check('an empty string is an answer, not done', $o instanceof Answer && $o->message->body === '', get_class($o));
    $o = $me->call('#php-svc@localhost', 'x', new CallOptions(1.5, 'boom'));
    check('a throwing handler sends no done: the caller times out, acked', $o instanceof TimedOut && $o->acked, get_class($o));
    check('the default log names the failure, not its message', !str_contains((string) @file_get_contents(ini_get('error_log')), 'boom-secret')
        && str_contains((string) @file_get_contents(ini_get('error_log')), 'RuntimeException'), (string) @file_get_contents(ini_get('error_log')));
    check('the server stopped after maxMessages', reap($srv) === 0);
    $o = $me->call('#php-slow@localhost', 'x', new CallOptions(0.5));
    check('nobody serving: TimedOut, not acked', $o instanceof TimedOut && !$o->acked, get_class($o));
    client('#php-slow@localhost')->consume(new ConsumeOptions(wait: 1));
    // Answered into a queue only the caller reads, for when its own inbox is shared.
    $srv = child(function () {
        client('#php-svc@localhost')->serve(fn(Message $m) => 'via queue', new ServeOptions(maxMessages: 1, poll: 1));
    });
    $o = $me->call('#php-svc@localhost', 'x', new CallOptions(5, 'q', replyTo: 'php-replies@localhost'));
    check('a call with replyTo is answered in that inbox', $o instanceof Answer && $o->message->body === 'via queue'
        && $o->message->to === 'php-replies@localhost', $o instanceof Answer ? $o->message->to : get_class($o));
    reap($srv);
});

section('shared send and call cases', function () use ($me, $owner, $dir) {
    // The shared cases, with their fixture names mapped to this daemon's.
    $cases = function (string $file) use ($owner): array {
        $base = getenv('AB_CLIENT_CONFORMANCE_CASES') ?: __DIR__ . '/../../../testdata/client-conformance';
        $rows = json_decode(strtr((string) file_get_contents("$base/$file.json"),
            ['#caller@fixture' => $owner, '#worker@fixture' => '#php-svc@localhost']), true);
        check("the shared $file cases are not empty", is_array($rows) && $rows !== []);
        return $rows;
    };
    $svc = client('#php-svc@localhost');
    foreach ($cases('send') as $k) {
        $r = $k['request'];
        $id = $me->send($r['to'], $r['body'], new SendOptions($r['topic'] ?? null, $r['tag'] ?? null, ask: $r['ask'] ?? null));
        $m = $svc->consume(new ConsumeOptions(wait: 2));
        $want = $k['expected'];
        // Every expected key is read, so a field added to the fixtures cannot go unchecked.
        $field = fn(?Message $m, string $key) => match ($key) {
            'from' => $m?->from, 'to' => $m?->to, 'body' => $m?->body,
            'topic' => $m?->topic ?? '', 'tag' => $m?->tag ?? '', 'ask' => $m?->ask ?? '',
            default => "no adapter for expected field $key",
        };
        $same = $m !== null && $m->id === $id;
        foreach ($want as $key => $value) { $same = $same && $field($m, $key) === $value; }
        check("send {$k['id']}: arrives field for field under the returned ID", $same, var_export($m, true));
    }
    $rows = $cases('call');
    $log = "$dir/php-shared-call-handled";
    @unlink($log);
    $srv = child(function () use ($rows, $log) {
        client('#php-svc@localhost')->serve(function (Message $m) use ($rows, $log) {
            foreach ($rows as $k) {
                if ($k['request']['body'] === $m->body) {
                    file_put_contents($log, "{$m->id} {$m->body}\n", FILE_APPEND);
                    return $k['answer'];
                }
            }
            return 'no case for ' . $m->body;
        }, new ServeOptions(maxMessages: count($rows), poll: 1));
    });
    $handled = [];
    foreach ($rows as $k) {
        $r = $k['request'];
        $o = $me->call($r['to'], $r['body'], new CallOptions(5, $r['topic'] ?? null));
        $want = $k['expected'];
        $same = $o instanceof Answer;
        foreach ($want as $key => $value) {
            $got = !$same ? null : match ($key) {
                'from' => $o->message->from, 'to' => $o->message->to, 'body' => $o->message->body,
                'topic' => $o->message->topic ?? '', 'tag' => $o->message->tag ?? '', 'ask' => $o->message->ask ?? '',
                default => "no adapter for expected field $key",
            };
            $same = $same && $got === $value;
        }
        check("call {$k['id']}: the handler's answer", $same, $o instanceof Answer ? json_encode($o->message->body) : get_class($o));
        $handled[$k['id']] = $o instanceof Answer ? $o->message->re : null;
    }
    reap($srv);
    $seen = array_filter(explode("\n", (string) @file_get_contents($log)));
    check('each call ran its handler once', count($seen) === count($rows), json_encode($seen));
    foreach ($rows as $k) {
        check("call {$k['id']}: the answer names the request it answered",
            in_array("{$handled[$k['id']]} {$k['request']['body']}", $seen, true), json_encode($seen));
    }
});

section('self call', function () {
    // The shared cases, with their fixture name mapped to this daemon's.
    $dir = getenv('AB_CLIENT_CONFORMANCE_CASES') ?: __DIR__ . '/../../../testdata/client-conformance';
    $cases = json_decode(str_replace('#caller@fixture', '#php-svc@localhost',
        (string) file_get_contents("$dir/self-call.json")), true);
    check('the shared self-call cases are not empty', is_array($cases) && $cases !== []);
    foreach ($cases as $k) {
        ['to' => $to, 'body' => $body, 'topic' => $topic] = $k['request'];
        $want = $k['expected'];
        foreach ([null, 'php-replies@localhost'] as $replyTo) {
            $what = $k['id'] . ($replyTo === null ? '' : " via $replyTo");
            $svc = client($to);
            $out = null; $handled = [];
            // One message at a time: the self-call is made from inside a handler.
            $svc->send($to, '', new SendOptions('self-trigger', 'self-trigger'));
            $svc->serve(function (Message $m) use ($svc, $to, $body, $topic, $replyTo, $k, &$out, &$handled) {
                if ($m->topic === 'self-trigger') {
                    $out = $svc->call($to, $body, new CallOptions(5, $topic, replyTo: $replyTo));
                    return null;
                }
                $handled[] = "{$m->id} {$m->body}";
                return $k['answer'];
            }, new ServeOptions(maxMessages: 1, poll: 1));
            check("$what: the handler's answer, not the request", $out instanceof Answer
                && $out->message->body === $want['body'] && $out->message->from === $want['from']
                && $out->message->to === ($replyTo ?? $want['to']) && $out->message->topic === $want['topic'],
                $out === null ? 'none' : json_encode($out));
            check("$what: the handler ran once on the request it answers", count($handled) === 1 && $out instanceof Answer
                && $handled[0] === "{$out->message->re} $body", json_encode($handled));
        }
        $e = thrown(fn() => $svc->call($to, $body, new CallOptions(2, $topic)));
        check("{$k['id']} outside serve(): refused, never its own request as the answer",
            $e instanceof AgentBus\BusException, $e === null ? 'returned' : get_class($e));
    }
});

section('ask', function () use ($me) {
    $srv = child(function () {
        client('#php-svc@localhost')->serve(fn(Message $m) => 'ask=' . ($m->ask ?? '') . " body={$m->body}",
            new ServeOptions(maxMessages: 2, poll: 1));
    });
    $o = $me->call('#php-svc@localhost', '', new CallOptions(5, ask: 'help'));
    check('a call can ask: the ask travels as the field, with no body', $o instanceof Answer && $o->message->body === 'ask=help body=',
        $o instanceof Answer ? $o->message->body : get_class($o));
    $o = $me->call('#php-svc@localhost', 'work', new CallOptions(5));
    check('a plain call carries no ask', $o instanceof Answer && $o->message->body === 'ask= body=work',
        $o instanceof Answer ? $o->message->body : get_class($o));
    $e = thrown(fn() => $me->send('#php-svc@localhost', 'text', new SendOptions(ask: 'help')));
    check('an ask with a body is refused as invalid', $e instanceof Invalid && $e->status() === 400, $e ? get_class($e) : 'none');
    reap($srv);
});

section('serve', function () use ($me, $dir) {
    // A request whose caller already gave up is not run, and not acked.
    $me->send('#php-svc@localhost', 'late', new SendOptions('late', 'l1', wait: 0.05));
    usleep(200000);
    $ran = [];
    $svc = client('#php-svc@localhost');
    $n = $svc->serve(function (Message $m) use (&$ran) { $ran[] = $m->body; return 'x'; },
        new ServeOptions(idle: 1, poll: 0.5, log: fn() => null));
    check('a too-late request is not run', $ran === [] && $n === 0, json_encode($ran));
    owner('POST', '/send', ['to' => '#php-svc@localhost', 'body' => 'a late answer', 're' => 'some-earlier-call']);
    $n = $svc->serve(function (Message $m) use (&$ran) { $ran[] = $m->body; return 'x'; },
        new ServeOptions(idle: 1, poll: 0.5, log: fn() => null));
    check('an answer that arrives late is no request', $ran === [] && $n === 0, json_encode($ran));
    check('nor acked', $me->consume(new ConsumeOptions(topic: 'late', tag: 'l1', wait: 0)) === null);
    // SIGTERM ends the loop after the message in hand; it does not cut it short.
    $flag = "$dir/php-sigterm";
    $pid = child(function () use ($flag) {
        client('#php-svc@localhost')->serve(function () use ($flag) {
            posix_kill(posix_getpid(), SIGTERM);
            usleep(300000);
            file_put_contents($flag, 'finished');
            return 'done anyway';
        }, new ServeOptions(poll: 0.5));
    });
    $o = $me->call('#php-svc@localhost', 'x', new CallOptions(5));
    check('SIGTERM lets the handler finish and answer', $o instanceof Answer && $o->message->body === 'done anyway', get_class($o));
    check('then the server returns', reap($pid, 3) === 0 && @file_get_contents($flag) === 'finished');
    // A pool: two processes serving one name, each saying so.
    $pids = [];
    foreach ([1, 2] as $i) {
        $pids[] = child(function () use ($i) {
            client('#php-pool@localhost')->serve(fn() => "w$i", new ServeOptions(maxMessages: 1, share: true, poll: 1));
        });
    }
    usleep(300000);
    $seen = [];
    foreach ([1, 2] as $_) {
        $o = $me->call('#php-pool@localhost', 'x', new CallOptions(5));
        $seen[] = $o instanceof Answer ? $o->message->body : get_class($o);
    }
    sort($seen);
    check('a shared pool: both workers serve', $seen === ['w1', 'w2'], json_encode($seen));
    foreach ($pids as $p) { reap($p); }
});

section('consume during serve', function () {
    $svc = client('#php-svc@localhost');
    $e = null;
    $me = Bus::connect(getenv('AB_TEST_ADDR'), getenv('AB_TEST_OWNER_TOKEN'), getenv('AB_TEST_OWNER'));
    $me->send('#php-svc@localhost', 'in');
    $svc->serve(function () use ($svc, &$e) { $e = thrown(fn() => $svc->consume()); return null; },
        new ServeOptions(maxMessages: 1, poll: 1));
    check('consume() inside serve() is refused, not raced', $e instanceof InvalidArgument, $e ? get_class($e) : 'none');
});

section('publish', function () use ($me) {
    $other = client('#php-other@localhost');
    $me->publish('php-news@localhost', 'extra');
    $m = $other->consume(new ConsumeOptions(wait: 2));
    check('a publication reaches the Deliver-To list', $m?->body === 'extra' && $m->originalTo === 'php-news@localhost',
        json_encode($m));
    $other->unsubscribe('php-news@localhost');
    $e = thrown(fn() => $me->publish('php-news@localhost', 'gone'));
    check('after unsubscribe nobody takes it: refused', $e instanceof NotFound, $e ? get_class($e) . $e->getMessage() : 'none');
});

section('private values', function () use ($me, $addr, $owner) {
    owner('POST', '/configure', ['name' => '#php-svc@localhost', 'config' => ['a' => 1, 'empty' => new stdClass(), 'list' => [], 'big' => 9007199254740993]]);
    owner('POST', '/secret', ['name' => '#php-svc@localhost', 'secret' => "API_KEY=s3cr3t-value\n# note\nOTHER='two words'\n"]);
    $svc = client('#php-svc@localhost');
    $r = $svc->me();
    $c = $r->config();
    check('config is the stored JSON, {} and [] kept apart', $c->a === 1 && $c->empty instanceof stdClass && $c->list === [], json_encode($c));
    check('a big int survives', $c->big === 9007199254740993);
    $c->a = 99;
    $c2 = $r->config(); // the first answer from the cache, not from the fetch
    $c2->a = 98;
    check('changing the answer does not change the cache', $r->config()->a === 1);
    owner('POST', '/configure', ['name' => '#php-svc@localhost', 'config' => ['a' => 2]]);
    check('the cache is kept until refresh', $r->config()->a === 1);
    $r->refresh();
    check('refresh reads it again', $r->config()->a === 2);
    $s = $r->secret();
    check('the secret is its bytes', $s->bytes() === "API_KEY=s3cr3t-value\n# note\nOTHER='two words'\n");
    check('and its env', $s->env() === ['API_KEY' => 's3cr3t-value', 'OTHER' => 'two words'], json_encode($s->env()));
    $leaks = [(string) $s, print_r($s, true), var_export($s, true), json_encode((array) $s), var_export((array) $s, true)];
    ob_start(); var_dump($s); $leaks[] = ob_get_clean();
    check('no printing of it shows the value', !str_contains(implode('|', $leaks), 's3cr3t'), implode('|', $leaks));
    $credentialClient = Bus::connect($addr, 'client-token-sentinel');
    ob_start(); var_dump($credentialClient); $printedClient = ob_get_clean();
    check('client bearer is absent from diagnostic printing', !str_contains($printedClient . print_r($credentialClient, true), 'client-token-sentinel'));
    $credentialClient->close();
    check('serializing it is refused', thrown(fn() => serialize($s)) instanceof LogicException);
    check('an absent config is null', $me->record('#php-slow@localhost')->config() === null);
    $e = thrown(fn() => $me->record('#php-slow@localhost')->secret());
    check('an absent secret is NotFound', $e instanceof NotFound, $e ? get_class($e) : 'none');
    $bob = client('bob@localhost');
    $e = thrown(fn() => $bob->record('#php-svc@localhost')->config());
    check('somebody on the allow list only may not read it', $e instanceof Forbidden, $e ? get_class($e) . ' ' . $e->getMessage() : 'none');
    $e = thrown(fn() => $me->record('php-news@localhost')->config());
    check('a kind holding no private values is a 400', $e instanceof Invalid, $e ? get_class($e) : 'none');
    $e = thrown(fn() => $bob->record('#php-svc@localhost')->secret());
    check('nor its secret', $e instanceof Forbidden, $e ? get_class($e) : 'none');
});

section('kv', function () use ($me) {
    $kv = $me->record('#php-svc@localhost')->kv;
    $kv->set('s', "plain text");
    $kv->set('n', 41, Kind::Int);
    $kv->set('j', (object) ['jobs' => [], 'meta' => new stdClass()], Kind::Json);
    check('a string comes back', $kv->get('s') === 'plain text');
    check('an int is an int', $kv->get('n', Kind::Int) === 41);
    $j = $kv->get('j', Kind::Json);
    check('JSON keeps {} and []', $j->jobs === [] && $j->meta instanceof stdClass, json_encode($j));
    check('kinds are separate names', thrown(fn() => $kv->get('n')) instanceof NotFound);
    $bytes = "\xff\x00\xfe raw";
    $kv->set('b', $bytes);
    check('bytes that are not UTF-8 survive', $kv->get('b') === $bytes);
    check('inc answers the new value', $kv->inc('n') === 42 && $kv->inc('n', -2) === 40);
    check('inc on an absent name counts from zero', $kv->inc('fresh', 5) === 5);
    $e = thrown(fn() => $kv->set('s', 'again', how: How::Add));
    check('add refuses a present name: Exists', $e instanceof Exists, $e ? get_class($e) : 'none');
    $e = thrown(fn() => $kv->set('nope', 'x', how: How::Replace));
    check('replace refuses an absent one: NotFound', $e instanceof NotFound, $e ? get_class($e) : 'none');
    $kv->set('s', 'replaced', how: How::Replace);
    check('replace replaces', $kv->get('s') === 'replaced');
    $r = $kv->json('j', Op::push('jobs', 'a'), Op::push('jobs', 'b'), Op::inc('count', 3), Op::shift('jobs'));
    check('ops report what they did', count($r) === 4 && $r[3]->value === 'a' && $r[2]->value === 3, json_encode($r));
    check('and applied in order', $kv->get('j', Kind::Json)->jobs === ['b']);
    $e = thrown(fn() => $kv->json('j', Op::push('more', 1), Op::inc('jobs', 1)));
    check('one bad op refuses the list', $e instanceof Refusal, $e ? get_class($e) : 'none');
    check('and applies none of it', !isset($kv->get('j', Kind::Json)->more));
    $l = $kv->list();
    check('list answers by kind', in_array('s', array_column($l['string'], 'name'), true)
        && in_array('n', array_column($l['int'], 'name'), true), json_encode($l));
    check('delete says whether there was one', $kv->delete('s') && !$kv->delete('s'));
    check('a PHP float is no int', thrown(fn() => $kv->set('f', 1.5, Kind::Int)) instanceof InvalidArgument);
    $e = thrown(fn() => client('bob@localhost')->record('#php-svc@localhost')->kv->get('n', Kind::Int));
    check('the allow list does not grant the store', $e instanceof Forbidden || $e instanceof NotFound, $e ? get_class($e) : 'none');
});

section('locks', function () use ($me, $dir) {
    $locks = $me->record('#php-svc@localhost')->locks;
    $lease = $locks->acquire('build', 30);
    check('a held lock is listed with its holder', ($locks->holders()['build']['holder'] ?? '') === getenv('AB_TEST_OWNER'));
    $e = thrown(fn() => $locks->tryAcquire('build', 30));
    check('the same name taking it again is Busy at once', $e instanceof Busy, $e ? get_class($e) : 'none');
    // Another name waits, and gets it when it is released.
    $svc = '#php-svc@localhost';
    $t0 = microtime(true);
    $pid = child(function () use ($svc, $dir) {
        $l = client($svc)->me()->locks->acquire('build', 30, 5);
        file_put_contents("$dir/php-lock", 'got');
        $l->release();
    });
    usleep(400000);
    check('the waiter does not have it yet', !file_exists("$dir/php-lock"));
    check('release sends', $lease->release());
    check('the waiter gets it', reap($pid, 5) === 0 && @file_get_contents("$dir/php-lock") === 'got');
    check('a second release sends nothing', !$lease->release());
    $short = $locks->acquire('short', 0.3);
    usleep(400000);
    check('past its ttl a lease refuses to extend', thrown(fn() => $short->extend(10)) instanceof LeaseExpired);
    $r = null;
    $e = thrown(function () use ($short, &$r) { $r = $short->release(); });
    check('and to release, sending nothing', $e === null && $r === false, $e ? get_class($e) : var_export($r, true));
    $other = client($svc)->me()->locks;
    $other->acquire('taken', 30);
    $e = thrown(fn() => $locks->tryAcquire('taken', 30));
    check('held by somebody else: Busy', $e instanceof Busy, $e ? get_class($e) : 'none');
    check('force release answers the displaced holder', $locks->forceRelease('taken') === $svc);
    check('and nobody holds it now', !isset($locks->holders()['taken']));
    $e = thrown(fn() => $locks->release('never'));
    check('releasing what nobody holds is NotFound', $e instanceof NotFound, $e ? get_class($e) : 'none');
    $l = $locks->acquire('ext', 1);
    $l->extend(30);
    usleep(1200000);
    check('extend keeps it past the first ttl', isset($locks->holders()['ext']));
    $l->release();
});

section('uncertain writes', function () use ($dir) {
    // A server that reads the request and hangs up without answering.
    $sock = "$dir/php-dead.sock";
    @unlink($sock);
    @unlink("$sock.count");
    $srv = stream_socket_server("unix://$sock");
    $pid = child(function () use ($srv) {
        for ($i = 0; $i < 2; $i++) { $c = stream_socket_accept($srv, 5); fread($c, 65536); fclose($c); }
    });
    $b = Bus::connect($sock, 'x', null, 2.0);
    $e = thrown(fn() => $b->send('#a@localhost', 'x'));
    check('a send whose answer never came is UncertainOutcome', $e instanceof UncertainOutcome, $e ? get_class($e) . ' ' . $e->getMessage() : 'none');
    $e = thrown(fn() => $b->consume(new ConsumeOptions(wait: 0)));
    check('a consume that fails after leaving is uncertain: it may have taken one', $e instanceof UncertainOutcome, $e ? get_class($e) : 'none');
    reap($pid);
    $e = thrown(fn() => Bus::connect("$dir/none.sock", 'x')->send('#a@localhost', 'x'));
    check('no daemon at all is Transport: nothing left', $e instanceof Transport, $e ? get_class($e) : 'none');
});


section('fake request framing', function () use ($dir) {
    $sock = "$dir/php-framing.sock";
    $pid = fake($sock, fn($line) => [0, 200, '{}']);
    $c = stream_socket_client("unix://$sock");
    try {
        $body = "é";
        fwrite($c, "POST /send HTTP/1.1\r\nContent-Length: " . strlen($body) . "\r\n\r\n");
        $read = [$c]; $write = $except = [];
        $early = stream_select($read, $write, $except, 0, 100000);
        check('the fake does not answer a POST before its body arrives', $early === 0);
        if ($early !== 0) { return; }
        fwrite($c, substr($body, 0, 1));
        $read = [$c];
        $early = stream_select($read, $write, $except, 0, 100000);
        check('the fake waits for every Content-Length byte', $early === 0);
        if ($early !== 0) { return; }
        fwrite($c, substr($body, 1));
        stream_set_timeout($c, 2);
        $response = stream_get_contents($c);
        check('the fake answers the completed fragmented POST', str_starts_with($response, 'HTTP/1.1 200 '));
    } finally { fclose($c); unfake($pid); }
});

section('budgets and receipts', function () use ($dir) {
    $sock = "$dir/php-fake.sock";
    // A send slower than the whole call's budget: cut at the budget, and uncertain.
    $pid = fake($sock, fn($line) => str_starts_with($line, 'POST /send') ? [0.6, 200, '{"message_id":"m1"}'] : [0, 204, '']);
    $t0 = microtime(true);
    $e = thrown(fn() => Bus::connect($sock, 'x')->call('#a@x', 'b', new CallOptions(0.15)));
    $took = microtime(true) - $t0;
    check('a send slower than the call budget ends at the budget', $took < 0.45, sprintf('%.3fs', $took));
    check('as an uncertain write', $e instanceof UncertainOutcome, $e ? get_class($e) : 'none');
    unfake($pid);
    // A fast send and an answer that comes after the budget: TimedOut, not Answer.
    $pid = fake($sock, function ($line) {
        if (str_starts_with($line, 'POST /send')) { return [0, 200, '{"message_id":"m1"}']; }
        parse_str(parse_url(explode(' ', $line)[1], PHP_URL_QUERY) ?? '', $q);
        return [0.6, 200, json_encode(['message_id' => 'a1', 'from' => '#a@x', 'to' => 'me', 'body' => 'late', 'topic' => $q['topic'] ?? '', 'tag' => $q['tag'] ?? '', 're' => 'm1'])];
    });
    $t0 = microtime(true);
    $o = Bus::connect($sock, 'x')->call('#a@x', 'b', new CallOptions(0.15));
    $took = microtime(true) - $t0;
    check('an answer later than the budget is no answer', $o instanceof TimedOut, get_class($o));
    check('and the call ends at its budget', $took < 0.45, sprintf('%.3fs', $took));
    unfake($pid);
    // A done receipt that names no message, or another one, is not this call's.
    $receipt = function (?string $re) {
        return function ($line, $i) use ($re) {
            if (str_starts_with($line, 'POST /send')) { return [0, 200, '{"message_id":"m1"}']; }
            parse_str(parse_url(explode(' ', $line)[1], PHP_URL_QUERY) ?? '', $q);
            $e = ['message_id' => "r$i", 'from' => '#a@x', 'to' => 'me', 'body' => '', 'receipt' => 'done', 'topic' => $q['topic'] ?? '', 'tag' => $q['tag'] ?? ''];
            if ($re !== null) { $e['re'] = $re; }
            return [0.02, 200, json_encode($e)];
        };
    };
    foreach ([null => 'no message', 'other' => 'another message'] as $re => $what) {
        $pid = fake($sock, $receipt($re === '' ? null : $re));
        $o = Bus::connect($sock, 'x')->call('#a@x', 'b', new CallOptions(0.3));
        check("a done receipt naming $what is not this call's", $o instanceof TimedOut, get_class($o));
        unfake($pid);
    }
    $pid = fake($sock, $receipt('m1'));
    $o = Bus::connect($sock, 'x')->call('#a@x', 'b', new CallOptions(2));
    check("one naming this call's message is", $o instanceof Done, get_class($o));
    unfake($pid);
    // A wait cut short long before the budget is a failure, not a timeout.
    $pid = fake($sock, fn($line) => str_starts_with($line, 'POST /send') ? [0, 200, '{"message_id":"m1"}'] : [0, 'hangup', '']);
    $t0 = microtime(true);
    $e = thrown(fn() => Bus::connect($sock, 'x')->call('#a@x', 'b', new CallOptions(2)));
    check('a wait lost early is UncertainOutcome, not TimedOut', $e instanceof UncertainOutcome && microtime(true) - $t0 < 1.0,
        $e ? get_class($e) : 'returned');
    unfake($pid);
    $pid = fake($sock, fn() => [0, 200, '{"message_id":"m1"}']);
    $e = thrown(fn() => Bus::connect($sock, 'x')->call('#a@x', 'b', new CallOptions(0)));
    check('a call with no time is refused before anything is sent', $e instanceof InvalidArgument && seen($sock) === 0,
        ($e ? get_class($e) : 'none') . ' ' . seen($sock));
    unfake($pid);
    // A config answer that does not decode is not cached.
    $pid = fake($sock, fn($line, $i) => $i === 0 ? [0, 200, '{bad'] : [0, 200, '{"ok":true}'], 2);
    $r = Bus::connect($sock, 'x')->record('#a@x');
    $first = thrown(fn() => $r->config());
    $second = null;
    $e = thrown(function () use ($r, &$second) { $second = $r->config(); });
    check('a config that does not decode is not cached', $first instanceof JsonException && $e === null && ($second->ok ?? false) === true,
        ($first ? get_class($first) : 'none') . ' / ' . ($e ? get_class($e) : json_encode($second)));
    check('the second read asked again', seen($sock) === 2, (string) seen($sock));
    unfake($pid);
});

section('close', function () use ($me) {
    $b = client('#php-other@localhost');
    $r = $b->record('#php-other@localhost');
    $b->close();
    check('a send after close is refused', thrown(fn() => $b->send('#php-svc@localhost', 'x')) instanceof AgentBus\BusException);
    check('and so is a Record it handed out', thrown(fn() => $r->kv->inc('n')) instanceof AgentBus\BusException);
    $own = client('#php-svc@localhost');
    $rec = $own->me();
    $rec->config();
    $rec->secret();
    $own->close();
    check('a closed client answers no cached config', thrown(fn() => $rec->config()) instanceof AgentBus\BusException);
    check('nor a cached secret', thrown(fn() => $rec->secret()) instanceof AgentBus\BusException);
    $svc = client('#php-svc@localhost');
    $me->send('#php-svc@localhost', 'one');
    $me->send('#php-svc@localhost', 'two');
    $n = null;
    $e = thrown(function () use ($svc, &$n) {
        $n = $svc->serve(function () use ($svc) { $svc->close(); return 'bye'; }, new ServeOptions(poll: 0.5));
    });
    check('closing from a handler ends serve() after that message', $e === null && $n === 1, $e ? get_class($e) . ' ' . $e->getMessage() : (string) $n);
    client('#php-svc@localhost')->consume(new ConsumeOptions(wait: 1));
    check('cloning a Secret is refused', thrown(fn() => clone (new Secret('x'))) instanceof LogicException);
});

echo "passed $pass, failed $fail\n";
exit($fail === 0 ? 0 : 1);
