# Database proxy

📌 **TL;DR:** A runner-hosted Python proxy that runs one SQL statement per call against one MySQL,
PostgreSQL or ClickHouse database. `read` is enforced by the database, never by parsing SQL.

Password masking covers text cells only; numeric and binary cells are not masked.
Account names in results are not masked, so `SELECT current_user` shows the account.

See [calls, roles and installation](guide.md).
Tests start a MariaDB from the installed server and PostgreSQL and ClickHouse in rootless podman
(digest-pinned, loopback only); a missing fixture fails the run.
