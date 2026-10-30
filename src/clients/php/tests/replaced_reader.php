<?php
// J43: a PHP reader that gives up on its poll and polls again, over https to a
// daemon that offers HTTP/2, with two messages sent between. run.php starts it
// with curl.cainfo naming the test certificate and reads the counts it prints.
// Env: AB_TEST_HTTPS, AB_TEST_ADDR (the socket, for the sender),
// AB_TEST_OWNER, AB_TEST_OWNER_TOKEN, AB_REPLACED_READER and its token
// AB_REPLACED_TOKEN, AB_REPLACED_RUNS.
declare(strict_types=1);

require __DIR__ . '/../src/autoload.php';

use AgentBus\{Bus, Http};

$reader = getenv('AB_REPLACED_READER');
$runs = (int) getenv('AB_REPLACED_RUNS');
$http = new Http(getenv('AB_TEST_HTTPS'), getenv('AB_REPLACED_TOKEN'), 10.0);
$sender = Bus::connect(getenv('AB_TEST_ADDR'), getenv('AB_TEST_OWNER_TOKEN'), getenv('AB_TEST_OWNER'), 10.0);

$http->json('GET', '/identity');
$other = $http->version();
$polls = [];
$lost = 0; $doubled = 0; $abandoned = 0;
for ($run = 0; $run < $runs; $run++) {
    // The old reader: a 30 s wait that curl abandons after 150 ms, leaving the
    // daemon a waiter whose caller is gone.
    try {
        $http->json('GET', '/consume', ['wait' => '30s'], null, true, 0.15);
    } catch (Throwable $e) {
        $abandoned++;
    }
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
echo json_encode(['runs' => $runs, 'abandoned' => $abandoned, 'lost' => $lost, 'doubled' => $doubled,
    'other' => $other, 'polls' => array_keys($polls)]), "\n";
