# Dotenv conformance data

`common.json` is consumed by the Go parser, daemon write checks, runner delivery
and Slack's TypeScript parser. Its first case contains the verbatim upstream
[`tests/.env`](https://github.com/motdotla/dotenv/blob/master/tests/.env) and
expected values from [`tests/test-parse.js`](https://github.com/motdotla/dotenv/blob/master/tests/test-parse.js), retrieved 2026-10-17.
The upstream fixture and parser are BSD-2-Clause licensed; see `LICENSE.dotenv`.

The remaining cases pin the bus contract's unreadable-line refusal, literal
evaluation syntax, later duplicate assignments and less common dotenv forms.
An accepted case's `want` is the assignment list in source order; later keys
replace earlier values when forming an environment. A refused case's `line`
names the first unreadable line. Private input must never appear in an error.
