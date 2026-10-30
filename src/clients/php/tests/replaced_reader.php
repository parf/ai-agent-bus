<?php
// J43: a PHP reader that gives up on its poll and polls again, over https to a
// daemon that offers HTTP/2, with two messages sent between. run.php starts it
// with curl.cainfo naming the test certificate and reads the counts it prints.
// Env: AB_TEST_HTTPS, AB_TEST_ADDR (the socket, for the sender and the
// watcher), AB_TEST_OWNER, AB_TEST_OWNER_TOKEN, AB_REPLACED_READER and its
// token AB_REPLACED_TOKEN, AB_REPLACED_RUNS. AB_REPLACED_STATUS_ADDR, a probe's,
// points only the reader's own status reads elsewhere, so they fail.
declare(strict_types=1);

require __DIR__ . '/../src/autoload.php';

use AgentBus\{Bus, Http, UncertainOutcome};

/** The daemon's count of waiting reads, asked over $socket as the owner; -1 when unreadable. */
function waiting(string $socket): int
{
    $c = curl_init('http://localhost/status');
    curl_setopt_array($c, [CURLOPT_UNIX_SOCKET_PATH => $socket, CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HTTPHEADER => ['X-Agent-Bus-Token: ' . getenv('AB_TEST_OWNER_TOKEN')], CURLOPT_TIMEOUT => 5]);
    $s = json_decode((string) curl_exec($c), true);
    return is_array($s) && is_int($s['waiting'] ?? null) ? $s['waiting'] : -1;
}

// The watcher: for each line, a count of waiting reads, answers "seen" once the
// daemon holds more than that, or "missed" when none came within 5 s. The reader cannot ask
// itself: it is blocked in the very poll being watched. An unreadable count, as
// baseline or reading, is never seen: any count would exceed it.
if (($argv[1] ?? '') === 'watch') {
    while (($line = fgets(STDIN)) !== false) {
        $before = (int) $line;
        $until = microtime(true) + 5;
        $now = waiting(getenv('AB_TEST_ADDR'));
        while ($before >= 0 && $now <= $before && microtime(true) < $until) { usleep(1000); $now = waiting(getenv('AB_TEST_ADDR')); }
        echo $before >= 0 && $now > $before ? "seen\n" : "missed\n";
        fflush(STDOUT);
    }
    exit(0);
}

$reader = getenv('AB_REPLACED_READER');
$runs = (int) getenv('AB_REPLACED_RUNS');
$http = new Http(getenv('AB_TEST_HTTPS'), getenv('AB_REPLACED_TOKEN'), 10.0);
$sender = Bus::connect(getenv('AB_TEST_ADDR'), getenv('AB_TEST_OWNER_TOKEN'), getenv('AB_TEST_OWNER'), 10.0);
$watcher = proc_open([PHP_BINARY, __FILE__, 'watch'], [0 => ['pipe', 'r'], 1 => ['pipe', 'w']], $pipes);

$http->json('GET', '/identity');
$other = $http->version();
$status = getenv('AB_REPLACED_STATUS_ADDR') ?: getenv('AB_TEST_ADDR');
$polls = [];
$lost = 0; $doubled = 0; $abandoned = 0; $admitted = 0; $errors = [];
for ($run = 0; $run < $runs; $run++) {
    // The daemon's own agents hold reads too: the old poll is one more.
    $before = waiting($status);
    if ($before < 0) { $errors[] = "run $run: the daemon's count of waiting reads is unreadable"; break; }
    fwrite($pipes[0], "$before\n");
    // The old reader: a 30 s wait that curl abandons after 300 ms, leaving the
    // daemon a waiter whose caller is gone. Only curl's own timeout (28) is
    // that; any other failure means no poll was abandoned.
    try {
        $http->json('GET', '/consume', ['wait' => '30s'], null, true, 0.3);
        $errors[] = "run $run: the 30 s poll answered within 300 ms";
    } catch (UncertainOutcome $e) {
        if ($e->getCode() === 28) { $abandoned++; } else { $errors[] = "run $run: " . $e->getMessage(); }
    }
    // A run whose old poll never waited proves nothing, and the next would wait 5 s again.
    if (trim((string) fgets($pipes[1])) !== 'seen') { $errors[] = "run $run: the old poll was never waiting"; break; }
    $admitted++;
    $sender->send($reader, 'one');
    $sender->send($reader, 'two');
    // Its replacement reads both.
    $got = [];
    for ($i = 0; $i < 2; $i++) {
        $e = $http->json('GET', '/consume', ['wait' => '2s'], null, true, 4.0);
        $polls[$http->version()] = true;
        if (is_array($e)) { $got[$e['body']] = ($got[$e['body']] ?? 0) + 1; }
    }
    // A second copy would still be queued: one more read, without waiting.
    $e = $http->json('GET', '/consume', ['wait' => '0s'], null, true, 4.0);
    if (is_array($e)) { $got[$e['body']] = ($got[$e['body']] ?? 0) + 1; }
    if (($got['one'] ?? 0) === 0 || ($got['two'] ?? 0) === 0) { $lost++; }
    if (($got['one'] ?? 0) > 1 || ($got['two'] ?? 0) > 1) { $doubled++; }
}
fclose($pipes[0]); proc_close($watcher);
echo json_encode(['runs' => $runs, 'abandoned' => $abandoned, 'admitted' => $admitted, 'lost' => $lost,
    'doubled' => $doubled, 'other' => $other, 'polls' => array_keys($polls), 'errors' => $errors]), "\n";
