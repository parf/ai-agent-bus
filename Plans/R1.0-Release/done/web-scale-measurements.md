# Web scale measurements — F.13.6

📌 **TL;DR:** The released TypeScript face was measured on a disposable daemon as the directory grew from 10 to 5,000 records. Only the Account page and the Owner's User page pass 1 MB of HTML; Groups reaches 0.23 MB; Activity stays flat. Every listing reads the whole visible directory from the bus (1.2 MB at 5,000). Nothing was built or changed; the choices are in [the recommendations](#recommendations).

## How it was measured

- **What ran:** a throwaway `agent-busd` and the web face (0.9.192, main `46b1c9ae`) on loopback, with a counting proxy on the daemon socket logging each call's response bytes. Nothing touched the live node; the daemon, its data and the builds were removed afterwards.
- **Data:** grown in place by the CLI to 10, 100, 1,000 and 5,000 steps. At 5,000 steps the directory held 3,500 agents, 1,000 queues and 500 groups (5,003 records with the Owner and built-ins), 500 messages sent and the audit entries of every registration. The Owner owns every record.
- **Method:** the Owner signs in, each page is fetched three times, the median server time is kept; HTML bytes are the response body, bus bytes the sum of the daemon responses for that page, loopback, uncompressed.
- **Host:** AMD Ryzen 9 5900X, Linux 7.2, other agents working at the time.

## Results

| Records | Page | HTML KB | Bus calls | Bus KB | Server ms |
|---|---|---|---|---|---|
| 13 | Groups | 9.2 | 4 | 4.0 | 1 |
| 13 | Account | 12.4 | 6 | 4.6 | 1 |
| 13 | Owner's User | 13.8 | 6 | 4.5 | 2 |
| 13 | Activity | 46.4 | 6 | 17.1 | 14 |
| 13 | Agents (a listing) | 15.5 | 4 | 3.9 | 2 |
| 103 | Groups | 13.1 | 4 | 25.4 | 2 |
| 103 | Account | 36.4 | 6 | 27.0 | 3 |
| 103 | Owner's User | 37.8 | 6 | 26.9 | 3 |
| 103 | Activity | 46.7 | 6 | 46.1 | 15 |
| 103 | Agents | 55.7 | 4 | 25.1 | 4 |
| 1,003 | Groups | 52.3 | 4 | 241.5 | 8 |
| 1,003 | Account | 277.7 | 6 | 253.9 | 16 |
| 1,003 | Owner's User | 279.1 | 6 | 253.8 | 14 |
| 1,003 | Activity | 50.2 | 6 | 337.2 | 36 |
| 1,003 | Agents | 74.5 | 4 | 238.0 | 10 |
| 5,003 | Groups | 228.5 | 4 | 1,208.6 | 40 |
| 5,003 | Account | 1,358.2 | 6 | 1,275.7 | 65 |
| 5,003 | Owner's User | 1,359.6 | 6 | 1,275.6 | 71 |
| 5,003 | Activity | 66.6 | 6 | 1,640.2 | 110 |
| 5,003 | Agents | 74.7 | 4 | 1,189.4 | 39 |

What the numbers say:

- **Groups HTML:** about 0.46 KB per group, linear.
- **Account and Owner's User HTML:** about 0.27 KB per owned record, linear; the one page past 1 MB.
- **Activity HTML:** flat (47 to 67 KB). Its bus bytes grow with the directory because it reads the listing, plus 87 KB (1,003 records) and 439 KB (5,003) from the days call, which the API already caps at 62 days.
- **Directory read:** every page that lists reads the whole visible directory in one call, `/ls`: 24 KB at 103 records, 237 KB at 1,003 and 1,189 KB at 5,003, about 0.24 KB a record. The number of calls stays constant. A listing's own HTML is paginated and stays at 75 KB.
- **Time:** the slowest page at 5,003 records answers in 110 ms (Activity), against 500 ms budgeted for the MVP gate.

## Recommendations

| Page | Measured at 5,003 records | Recommendation |
|---|---|---|
| Account and the Owner's User page | 1.36 MB of HTML, 0.27 KB per owned record | **Paginate** the owned-records list, as the listings are: the only page past 1 MB; most people own tens of records, the Owner of a large directory is the case |
| Groups | 0.23 MB at 500 groups, 0.46 KB per group | **Leave** below a few hundred groups; **paginate** with the same pager if groups pass about 1,000 (0.5 MB) |
| Activity | HTML flat at 67 KB; days call 439 KB, capped at 62 days | **Leave** |
| Directory bytes for a listing | 1.19 MB per listing page, constant calls, 40 ms | **Leave** until the directory passes about 10,000 records. A bound needs a daemon-side page or count in `/ls`, which is an API design and not settled; the web child cannot bound it alone |

No pagination or API design is chosen here. Acceptance for any change taken from this table: a check that fails when the bound is removed ([№F.13.6](../TODO.md#web-scale-follow-up-from-mvp)).
