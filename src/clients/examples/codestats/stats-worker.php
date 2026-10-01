<?php
// A code-stats worker in PHP: serves "stats" (one file's numbers) and "relay"
// (passes a message on round a ring of agents and brings the answer back).
// Run by codestats/run.sh with AGENT_BUS_ADDR, AGENT_BUS_TOKEN, AGENT_BUS_NAME
// and STATS_ROOT; it serves until SIGTERM.
declare(strict_types=1);

require getenv('EXAMPLES_ROOT') . '/src/clients/php/src/autoload.php';

use AgentBus\{Bus, Message, ServeOptions, CallOptions, Answer, Done, TimedOut};

$root = realpath(getenv('STATS_ROOT') ?: getenv('EXAMPLES_ROOT'));
$bus = Bus::connect();
$me = $bus->me();

function stats(string $path, string $root): array
{
    $real = realpath($path);
    // Only files under the root it was given: a request names a path, and a
    // worker that read anything it was asked for would be a way to read the host.
    if ($real === false || !str_starts_with($real, $root . '/') || !is_file($real)) {
        return ['error' => 'not a file under ' . $root];
    }
    $data = file_get_contents($real);
    $lines = substr_count($data, "\n") + ($data !== '' && !str_ends_with($data, "\n") ? 1 : 0);
    return ['path' => $real, 'bytes' => strlen($data), 'lines' => $lines,
            'todos' => substr_count($data, 'TODO'), 'sha256' => hash('sha256', $data)];
}

$handler = function (Message $m, Bus $b) use ($root, $me): string {
    $in = json_decode($m->body, true) ?: [];
    if ($m->topic === 'relay') {
        $in['hops'][] = 'php';
        $route = $in['route'] ?? [];
        if (!$route) { return json_encode($in); }
        $next = array_shift($route);
        $o = $b->call($next, json_encode(['route' => $route, 'hops' => $in['hops']]), new CallOptions(20, 'relay'));
        return match (true) {
            $o instanceof Answer => $o->message->body,
            $o instanceof Done => json_encode(['hops' => $in['hops'], 'error' => "$next finished without an answer"]),
            default => json_encode(['hops' => $in['hops'], 'error' => "$next did not answer in time"]),
        };
    }
    $out = ['worker' => 'php'] + stats((string) ($in['path'] ?? ''), $root);
    // Each worker keeps its own running count in its record's key-value store.
    if (!isset($out['error'])) { $out['served'] = $me->kv->inc('files'); }
    return json_encode($out);
};

fwrite(STDERR, "php stats worker {$bus->name()} serving files under $root\n");
$n = $bus->serve($handler, new ServeOptions(poll: 1));
fwrite(STDERR, "php stats worker stopped after $n requests\n");
