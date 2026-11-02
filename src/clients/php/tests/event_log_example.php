<?php
// An agent writes to its own private event log, its Owner reads and lists it,
// and nobody else may. Every call goes to the daemon's own #bus agent.
// Required by vectors.php, which runs it against the fixture and compares the
// printed lines; it is never pointed at a live bus.
declare(strict_types=1);

use AgentBus\{Bus, Answer, CallOptions};

/** The lines the example prints, for the fixture's {addr, tokens}. */
function eventLogExample(array $info): array
{
    $dial = fn(string $name): Bus => Bus::connect($info['addr'], $info['tokens'][$name], $name, 10.0);
    $agent = $dial('#caller@fixture');
    $owner = $dial('owner@fixture');
    $stranger = $dial('#outsider@fixture');
    // call waits for the answer; a send without wait is accepted and never
    // answered, so it proves only that the message was queued.
    $call = function (Bus $bus, string $body): array {
        $outcome = $bus->call('#bus', $body, new CallOptions(5));
        if (!$outcome instanceof Answer) { throw new RuntimeException("no answer to $body"); }
        return json_decode($outcome->message->body, true, 512, JSON_THROW_ON_ERROR);
    };
    $lines = [];
    $entry = $call($agent, 'write #caller@fixture alert disk full')['entry'];
    $lines[] = "written: {$entry['level']} {$entry['text']}";
    $alerts = $call($owner, 'read #caller@fixture')['rings']['alert'];
    $lines[] = 'owner reads: ' . count($alerts) . ' alert, newest ' . $alerts[0]['text'];
    $lines[] = 'owner lists: ' . $call($owner, 'list')['logs'][0]['log'];
    $lines[] = 'a third party: ' . $call($stranger, 'read #caller@fixture')['error']['code'];
    $lines[] = 'the writer reading its own log: ' . $call($agent, 'read #caller@fixture')['error']['code'];
    return $lines;
}
