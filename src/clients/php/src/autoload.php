<?php
// For use without Composer: require this file. Several small classes share a
// file, so this maps each to where it lives.
declare(strict_types=1);

spl_autoload_register(static function (string $class): void {
    static $map = [
        'Bus' => 'Bus', 'Record' => 'Record', 'Secret' => 'Secret', 'Http' => 'Http', 'Json' => 'Http',
        'Message' => 'Message', 'ReplyTo' => 'Message', 'Outcome' => 'Message', 'Answer' => 'Message',
        'Done' => 'Message', 'TimedOut' => 'Message',
        'SendOptions' => 'Options', 'ConsumeOptions' => 'Options', 'CallOptions' => 'Options', 'ServeOptions' => 'Options',
        'Kv' => 'Kv', 'Kind' => 'Kv', 'How' => 'Kv', 'Op' => 'Kv', 'OpResult' => 'Kv',
        'Locks' => 'Locks', 'Lease' => 'Locks',
    ];
    if (!str_starts_with($class, 'AgentBus\\')) { return; }
    $short = substr($class, 9);
    $file = $map[$short] ?? 'Errors';
    require_once __DIR__ . "/$file.php";
});
