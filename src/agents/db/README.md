# Database proxy

📌 **TL;DR:** A runner-hosted Python proxy that runs one SQL statement per call against one MySQL,
PostgreSQL or ClickHouse database. `read` is enforced by the database, never by parsing SQL.

See [calls, roles and installation](../../../docs/agents/db.md).
Tests start a MariaDB from the installed server and PostgreSQL and ClickHouse in rootless podman
(digest-pinned, loopback only); a missing fixture fails the run.
