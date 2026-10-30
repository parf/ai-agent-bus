<?php
declare(strict_types=1);

/** Every exec starts PHP anew; CLI ini overrides are not inherited. */
function testPhp(array $args): array
{
    return [PHP_BINARY, '-d', 'log_errors=0', '-d', 'display_errors=stderr',
        '-d', 'syslog.ident=' . (getenv('AB_PHP_JOURNAL_ID') ?: 'php'), ...$args];
}

// Refuse an unsafe direct invocation before any fixture can throw or log.
if (filter_var(ini_get('log_errors'), FILTER_VALIDATE_BOOLEAN)
    || ini_get('display_errors') !== 'stderr') {
    fwrite(STDERR, "FAIL PHP tests require log_errors=0 and display_errors=stderr\n");
    exit(1);
}
$sasl = getenv('SASL_PATH');
if (!$sasl || !is_dir($sasl) || scandir($sasl) !== ['.', '..']) {
    fwrite(STDERR, "FAIL PHP tests require an empty SASL_PATH directory\n");
    exit(1);
}
$marker = getenv('AB_PHP_JOURNAL_ID');
if ($marker && ini_get('syslog.ident') !== $marker) {
    fwrite(STDERR, "FAIL PHP test process lacks its shard journal marker\n");
    exit(1);
}
