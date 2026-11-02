<?php
// An Owner copies each alert of an agent's log to a queue, reads the one copy
// that arrives, and cancels. A copy is a notice from #bus: it is read, never
// acknowledged and never marked done. Required by vectors.php, which runs it
// against the fixture and compares the printed lines; it is never pointed at a
// live bus.
declare(strict_types=1);

use AgentBus\{Bus, Answer, CallOptions, ConsumeOptions};

/** The lines the example prints, for the fixture's {addr, tokens}. */
function eventSubscriptionExample(array $info): array
{
    $dial = fn(string $name): Bus => Bus::connect($info['addr'], $info['tokens'][$name], $name, 10.0);
    $agent = $dial('#caller@fixture');
    $owner = $dial('owner@fixture');
    $call = function (Bus $bus, string $body): array {
        $outcome = $bus->call('#bus', $body, new CallOptions(5));
        if (!$outcome instanceof Answer) { throw new RuntimeException("no answer to $body"); }
        return json_decode($outcome->message->body, true, 512, JSON_THROW_ON_ERROR);
    };
    $lines = [];
    $id = $call($owner, 'sub add jobs@fixture #caller@fixture alert')['id'];
    $lines[] = 'listed: ' . count($call($owner, 'sub list #caller@fixture')['subscriptions']) . ' subscription';
    $lines[] = 'delivered: ' . $call($agent, 'write #caller@fixture alert disk full')['deliveries']['sent'] . ' copy';
    $copy = $owner->consume(new ConsumeOptions(inbox: 'jobs@fixture', wait: 0.3));
    if ($copy === null) { throw new RuntimeException('no subscription copy'); }
    $entry = json_decode($copy->body, true, 512, JSON_THROW_ON_ERROR);
    $lines[] = "copy from {$copy->from}: {$entry['level']} {$entry['text']}";
    // A notice: nothing is acknowledged or marked done.
    $lines[] = 'one copy only: ' . ($owner->consume(new ConsumeOptions(inbox: 'jobs@fixture', wait: 0.3)) === null ? 'true' : 'false');
    $call($owner, "sub delete $id");
    $lines[] = 'after cancelling, matched: ' . $call($agent, 'write #caller@fixture alert after cancellation')['deliveries']['matched'];
    return $lines;
}
