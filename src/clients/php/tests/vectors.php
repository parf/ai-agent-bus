<?php
// The shared state, identity and refusal vectors (src/testdata/client-conformance:
// kv, locks, identity, refusals; not bigfile) through this client. Required by run.php,
// which supplies check(), thrown(), fake(), client() and the daemon.
declare(strict_types=1);

use AgentBus\{Bus, Kind, How, Op, Refusal, BusException};

/** The shared rows of one vector file, with the fixture names mapped to this daemon's. */
function vectorRows(string $file, array $names = []): array
{
    $base = getenv('AB_CLIENT_CONFORMANCE_CASES') ?: __DIR__ . '/../../../testdata/client-conformance';
    $rows = json_decode(strtr((string) file_get_contents("$base/$file.json"), $names), true);
    check("the shared $file vectors are not empty", is_array($rows) && $rows !== []);
    return $rows;
}

function vectorBytes(array $step): string
{
    if (isset($step['repeat'])) { return str_repeat($step['repeat']['text'], $step['repeat']['count']); }
    if (isset($step['value_b64'])) { return base64_decode($step['value_b64'], true); }
    return $step['value'] ?? '';
}

/** Runs $call under the step's expected refusal. [true, result] when it was meant to succeed; [false, null] after a matched refusal. */
function vectorStep(string $at, array $step, callable $call): array
{
    $want = $step['expect']['refused'] ?? null;
    $e = null; $result = null;
    try { $result = $call(); } catch (Throwable $t) { $e = $t; }
    if ($want === null) {
        check($at, $e === null, $e ? get_class($e) . ': ' . $e->getMessage() : '');
        return [$e === null, $result];
    }
    $ok = $e instanceof BusException;
    if ($ok && is_array($want)) {
        $ok = $e instanceof Refusal && $e->status() === $want['status']
            && (!isset($want['message_has']) || str_contains($e->getMessage(), $want['message_has']));
    }
    check("$at: refused as expected", $ok, $e ? get_class($e) . ' ' . $e->getMessage() : 'it succeeded');
    return [false, null];
}

section('shared kv vectors', function () use ($me) {
    foreach (vectorRows('kv') as $n => $case) {
        $record = "#php-vec-kv$n@localhost";
        $me->register(['kind' => 'agent', 'name' => $record, 'allow' => ['*']]);
        $kv = $me->record($record)->kv;
        foreach ($case['steps'] as $i => $step) {
            $at = "{$case['id']} step $i {$step['op']} " . ($step['name'] ?? '');
            $expect = $step['expect'] ?? [];
            $kind = Kind::from($step['kind'] ?? 'string');
            switch ($step['op']) {
                case 'set':
                    $value = match ($kind) {
                        Kind::Int => (int) $step['value'],
                        Kind::Json => json_decode($step['value']),
                        Kind::String => vectorBytes($step),
                    };
                    vectorStep($at, $step, fn() => $kv->set($step['name'], $value, $kind, How::from($step['how'] ?? 'set')));
                    break;
                case 'get':
                    [$ok, $got] = vectorStep($at, $step, fn() => $kv->get($step['name'], $kind));
                    if (!$ok) { break; }
                    if (isset($expect['value_b64'])) { $same = base64_encode($got) === $expect['value_b64']; }
                    elseif (isset($expect['length'])) { $same = strlen($got) === $expect['length']; }
                    elseif (array_key_exists('json', $expect)) { $same = json_decode(json_encode($got), true) == $expect['json']; }
                    elseif ($kind === Kind::Int) { $same = (string) $got === $expect['value']; }
                    else { $same = $got === $expect['value']; }
                    check("$at: reads back", $same, is_string($got) ? substr($got, 0, 60) : json_encode($got));
                    break;
                case 'fill':
                    $failed = null;
                    for ($k = 0; $k < $step['count'] && $failed === null; $k++) {
                        try { $kv->set($step['prefix'] . $k, 'v', $kind, How::Add); } catch (Throwable $t) { $failed = "$k: " . $t->getMessage(); }
                    }
                    check("$at: {$step['count']} names", $failed === null, (string) $failed);
                    break;
                case 'delete':
                    [$ok, $got] = vectorStep($at, $step, fn() => $kv->delete($step['name'], $kind));
                    if ($ok) { check("$at: deleted flag", $got === $expect['deleted'], var_export($got, true)); }
                    break;
                case 'inc':
                    [$ok, $got] = vectorStep($at, $step, fn() => $kv->inc($step['name'], (int) $step['by']));
                    if ($ok && isset($expect['value'])) { check("$at: counter", (string) $got === $expect['value'], (string) $got); }
                    break;
                case 'json':
                    $ops = array_map(fn($o) => match ($o['op']) {
                        'push' => Op::push($o['key'], $o['value']),
                        'unshift' => Op::unshift($o['key'], $o['value']),
                        'set' => Op::set($o['key'], $o['value']),
                        'inc' => Op::inc($o['key'], (int) $o['value']),
                        'unset' => Op::unset($o['key']),
                        'shift' => Op::shift($o['key']),
                        'pop' => Op::pop($o['key']),
                        'add_to_set' => Op::addToSet($o['key'], $o['value']),
                        'remove_from_set' => Op::removeFromSet($o['key'], $o['value']),
                    }, $step['ops']);
                    [$ok, $got] = vectorStep($at, $step, fn() => $kv->json($step['name'], ...$ops));
                    if ($ok && isset($expect['changed'])) {
                        check("$at: changed flags", array_map(fn($r) => $r->changed, $got) === $expect['changed'], json_encode($got));
                    }
                    break;
                default:
                    check("$at: known op", false, $step['op']);
            }
        }
        $me->record($record)->unregister();
    }
});

section('shared lock vectors', function () use ($me, $owner) {
    $worker = '#php-vec-worker@localhost';
    $me->register(['kind' => 'agent', 'name' => $worker, 'allow' => ['*']]);
    $buses = ['owner' => $me, 'worker' => client($worker)];
    foreach (vectorRows('locks', ['owner@fixture' => $owner, '#worker@fixture' => $worker]) as $n => $case) {
        $record = "#php-vec-lock$n@localhost";
        $me->register(['kind' => 'agent', 'name' => $record, 'allow' => ['*']]);
        foreach ($case['setup'] ?? [] as $setup) {
            check("setup {$setup} is known", $setup === 'worker-maintains-jobs');
            $me->record($record)->manage(['maintainers' => [$worker]]);
        }
        $leases = [];
        foreach ($case['steps'] as $i => $step) {
            $at = "{$case['id']} step $i {$step['as']} {$step['op']} " . ($step['name'] ?? '');
            $locks = $buses[$step['as']]->record($record)->locks;
            $expect = $step['expect'] ?? [];
            $ttl = $step['ttl_s'] ?? 0;
            switch ($step['op']) {
                case 'try':
                    [$ok, $lease] = vectorStep($at, $step, fn() => $locks->tryAcquire($step['name'], $ttl));
                    if ($ok) { $leases[] = $lease; }
                    break;
                case 'acquire':
                    [$ok, $lease] = vectorStep($at, $step, fn() => $locks->acquire($step['name'], $ttl, $step['wait_s'] ?? 0));
                    if ($ok) { $leases[] = $lease; }
                    break;
                case 'extend':
                    vectorStep($at, $step, fn() => $locks->extend($step['name'], $ttl));
                    break;
                case 'release':
                    vectorStep($at, $step, fn() => $locks->release($step['name']));
                    break;
                case 'force':
                    [$ok, $got] = vectorStep($at, $step, fn() => $locks->forceRelease($step['name']));
                    if ($ok) { check("$at: displaced holder", $got === $expect['displaced'], var_export($got, true)); }
                    break;
                case 'holders':
                    [$ok, $got] = vectorStep($at, $step, fn() => $locks->holders());
                    if ($ok) {
                        check("$at: holders", array_map(fn($h) => $h['holder'], $got) === $expect['holders'], json_encode(array_map(fn($h) => $h['holder'], $got)));
                    }
                    break;
                default:
                    check("$at: known op", false, $step['op']);
            }
        }
        // The vector held these on purpose; the library only logs a lease left held, and that is not this test's noise.
        foreach ($leases as $lease) { try { $lease->release(); } catch (Throwable) {} }
        $me->record($record)->unregister();
    }
});

/** A daemon whose /identity answers $identity raw, whose GET /ls answers $refuse, and any other path {"you":..}; every path is logged to "$sock.paths". */
function vectorDaemon(string $sock, string $identity, ?array $refuse = null): int
{
    @unlink("$sock.paths");
    return fake($sock, function ($line) use ($sock, $identity, $refuse) {
        $path = explode(' ', $line)[1] ?? '';
        file_put_contents("$sock.paths", $path . "\n", FILE_APPEND);
        $bare = explode('?', $path)[0];
        if ($bare === '/identity') { return [0, 200, $identity]; }
        if ($refuse !== null && $bare === '/ls') {
            return [0, $refuse['status'], $refuse['body'], isset($refuse['allow']) ? "Allow: {$refuse['allow']}\r\n" : ''];
        }
        return [0, 200, '{"you":"owner@fixture"}'];
    }, 200, false);
}
function vectorPaths(string $sock): array { return array_values(array_filter(explode("\n", (string) @file_get_contents("$sock.paths")))); }

section('shared identity vectors', function () use ($dir) {
    $sock = "$dir/php-vec-identity.sock";
    foreach (vectorRows('identity') as $case) {
        $pid = vectorDaemon($sock, $case['identity']);
        $bus = Bus::connect($sock, 't');
        $e = thrown(fn() => $bus->status());
        $expect = $case['expect'];
        if (isset($expect['fatal'])) {
            check("{$case['id']}: fatal naming it", $e !== null && str_contains($e->getMessage(), $expect['fatal']), $e ? $e->getMessage() : 'no error');
            check("{$case['id']}: the daemon saw only /identity", vectorPaths($sock) === ['/identity'], json_encode(vectorPaths($sock)));
        } else {
            $warns = $bus->warnings();
            check("{$case['id']}: serves", $e === null, $e ? $e->getMessage() : '');
            check("{$case['id']}: /identity was first", (vectorPaths($sock)[0] ?? '') === '/identity', json_encode(vectorPaths($sock)));
            if (isset($expect['warns'])) {
                check("{$case['id']}: warns", count($warns) === 1 && str_contains($warns[0], $expect['warns']), json_encode($warns));
            } else {
                check("{$case['id']}: silent", $warns === [], json_encode($warns));
            }
        }
        unfake($pid);
    }
});

section('shared refusal vectors', function () use ($dir) {
    $sock = "$dir/php-vec-refusal.sock";
    $current = '{"protocol":1,"protocol_min":1}';
    foreach (vectorRows('refusals') as $case) {
        if (isset($case['call'])) {
            $pid = vectorDaemon($sock, $current);
            $locks = Bus::connect($sock, 't')->record('jobs@localhost')->locks;
            $c = $case['call'];
            $e = thrown(fn() => match ($c['op']) {
                'acquire' => $locks->acquire($c['name'], $c['ttl_s'], $c['wait_s']),
                'try' => $locks->tryAcquire($c['name'], $c['ttl_s']),
                'extend' => $locks->extend($c['name'], $c['ttl_s']),
            });
            check("{$case['id']}: refused by the client", $e instanceof BusException && !($e instanceof Refusal), $e ? get_class($e) : 'accepted');
            $not = $case['expect']['not_requested'];
            check("{$case['id']}: $not never reached the daemon", !in_array($not, array_map(fn($p) => explode('?', $p)[0], vectorPaths($sock)), true), json_encode(vectorPaths($sock)));
            unfake($pid);
            continue;
        }
        $pid = vectorDaemon($sock, $case['identity'] === 'current' ? $current : '{}', $case['daemon']);
        $bus = Bus::connect($sock, 't');
        $e = thrown(fn() => $bus->list());
        $want = $case['expect'];
        check("{$case['id']}: status, kind and message", $e instanceof Refusal
            && $e->status() === $want['status'] && ($e->kind() ?? '') === $want['kind'] && $e->getMessage() === $want['message'],
            $e ? get_class($e) . ' ' . $e->getMessage() : 'no error');
        check("{$case['id']}: warns only for an old daemon", ($bus->warnings() !== []) === ($case['identity'] === 'old'), json_encode($bus->warnings()));
        unfake($pid);
    }
});
