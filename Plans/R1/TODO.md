# TODO R1

## Objective

Prepare the [proposed scope](README.md#scope) after MVP acceptance. Not started; there are no implementation waves committed yet.

## Next step

Owner confirms the scope and resolves [questions](QUESTIONS.md#open-questions). Existing decisions describe targets, not completed code.

## Dependencies

| Candidate work | Must precede it |
|---|---|
| Scoped credentials and encryption | Key lifecycle, offline receiver decryption and token grammar decisions |
| D.1–D.4 encryption carried from MVP | [Encryption acceptance](encryption-wave.md#d--the-bus-stops-reading-payloads); key lifecycle decisions |
| Federation | Namespace, record authenticity and clock decisions |
| Managed runner | Edge identity, config change behavior and dormant activation decisions; current method metadata work |
| Client libraries | Owner-approved protocol description |
| Dashboard extensions | The AUTH, health, stats, federation and runner data each view reports |

Name implementation waves and falsifiable acceptance after those choices. Crypto acceptance must test what the daemon cannot decrypt; federation acceptance must exercise distinct nodes.

## Backup acceptance

The [backup contract](runner.md#backing-it-up) is settled; its invocation remains
[Q33](QUESTIONS.md#open-questions). Implementation must restore an archive with
the intended user's key and refuse an unrelated key. Replace encryption with
plaintext output and the format/decryption check must fail; encrypt to the
wrong recipient and the intended-user restore must fail. Confirm backup creation
works with only the public key available.
