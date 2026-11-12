<?php
// A record's crons and one-shot sends, kept and fired by the daemon
// (docs/09-daemon-api.md#schedules). Answers are the daemon's JSON as arrays;
// the daemon judges every field and its refusal is a Refusal.
declare(strict_types=1);

namespace AgentBus;

final class Schedules
{
    /** @internal */
    public function __construct(private Http $t, private string $record) {}

    /**
     * Keeps a new schedule; give one of $expr, $in or $at. Answers
     * ['schedule' => ..., 'warning' => ...], warning set near a cap.
     */
    public function add(string $target, string $body, ?string $expr = null, ?string $in = null, ?string $at = null,
                        ?string $until = null, ?int $maxRuns = null, ?string $maxLate = null, bool $disabled = false): array
    {
        return $this->t->json('POST', '/schedule/add', [], ['record' => $this->record]
            + self::fields($target, $body, $expr, $in, $at, $until, $maxRuns, $maxLate, $disabled), true);
    }

    /** This record's schedules. */
    public function list(): array
    {
        return $this->t->json('GET', '/schedule/list', ['record' => $this->record])['schedules'] ?? [];
    }

    /** One schedule with its outcomes, newest first. */
    public function get(string $id): array
    {
        return $this->t->json('GET', '/schedule', ['id' => $id]);
    }

    /** Replaces a schedule's time, target, body and limits; the caller becomes its creator. */
    public function edit(string $id, string $target, string $body, ?string $expr = null, ?string $in = null, ?string $at = null,
                         ?string $until = null, ?int $maxRuns = null, ?string $maxLate = null, bool $disabled = false): array
    {
        return $this->t->json('POST', '/schedule/edit', [], ['id' => $id]
            + self::fields($target, $body, $expr, $in, $at, $until, $maxRuns, $maxLate, $disabled), true)['schedule'];
    }

    /** Lets a schedule fire again; the caller becomes its creator. */
    public function enable(string $id): array { return $this->post('/schedule/enable', $id)['schedule']; }

    /** Stops a schedule firing and keeps it. */
    public function disable(string $id): array { return $this->post('/schedule/disable', $id)['schedule']; }

    /** One firing now, answering its outcome; a lost answer is uncertain and never retried. */
    public function run(string $id): array { return $this->post('/schedule/run', $id)['outcome']; }

    /** Removes a schedule; one that is not there is a NotFound refusal. */
    public function delete(string $id): bool { return (bool) ($this->post('/schedule/delete', $id)['deleted'] ?? false); }

    private function post(string $path, string $id): array
    {
        return $this->t->json('POST', $path, [], ['id' => $id], true);
    }

    private static function fields(string $target, string $body, ?string $expr, ?string $in, ?string $at,
                                   ?string $until, ?int $maxRuns, ?string $maxLate, bool $disabled): array
    {
        $f = array_filter(['expr' => $expr, 'in' => $in, 'at' => $at, 'target' => $target, 'body' => $body,
            'until' => $until, 'max_runs' => $maxRuns, 'max_late' => $maxLate], fn($v) => $v !== null);
        if ($disabled) { $f['disabled'] = true; }
        return $f;
    }
}
