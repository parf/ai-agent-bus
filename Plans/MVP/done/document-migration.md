# Documentation migration

## Scope

2026-09-13: separated current MVP contracts from future release knowledge, moved completed PoC history, and retained original plan and decision snapshots. No runtime behavior changed in this rewrite.

## Section moves

| Original section | Destination |
|---|---|
| `docs/06-auth-role.md#bundle` | [owner](../../R1/auth.md#bundle) |
| `docs/06-auth-role.md#where-it-runs` | [owner](../../R1/auth.md#where-it-runs) |
| `docs/06-auth-role.md#topology` | [owner](../../R1/auth.md#topology) |
| `docs/06-auth-role.md#reference-deployment` | [owner](../../R1/auth.md#reference-deployment) |
| `docs/06-auth-role.md#consistency-window` | [owner](../../R1/auth.md#consistency-window) |
| `docs/06-auth-role.md#ssh-admin` | [owner](../../R1/auth.md#ssh-admin) |
| `docs/01-identity.md#groups-and-roles` | [owner](../../R1/identity.md#groups-and-roles) |
| `docs/01-identity.md#sigils` | [owner](../../R1/identity.md#sigils) |
| `docs/01-identity.md#delegation` | [owner](../../R1/identity.md#delegation) |
| `docs/01-identity.md#resolved-at-login` | [owner](../../R1/identity.md#resolved-at-login) |
| `docs/01-identity.md#ownership` | [owner](../../R1/identity.md#ownership) |
| `docs/01-identity.md#sealed-private-config` | [owner](../../R1/identity.md#sealed-private-config) |
| `docs/02-access.md#token-scope` | [owner](../../R1/access.md#token-scope) |
| `docs/02-access.md#key-modes` | [owner](../../R1/access.md#key-modes) |
| `docs/02-access.md#encrypted-sessions` | [owner](../../R1/access.md#encrypted-sessions) |
| `docs/02-access.md#key-confirmation` | [owner](../../R1/access.md#key-confirmation) |
| `docs/00-overview.md#chaining` | [owner](../../R1/federation.md#chaining) |
| `docs/03-services-and-topics.md#registry-sync` | [owner](../../R1/registry.md#registry-sync) |
| `docs/04-messaging.md#shared-locks` | [owner](../../R1/locks.md#shared-locks) |
| `docs/08-runner-role.md#what-the-runner-does` | [owner](../../R1/runner.md#what-the-runner-does) |
| `docs/08-runner-role.md#one-name-on-many-hosts` | [owner](../../R1/runner.md#one-name-on-many-hosts) |
| `docs/08-runner-role.md#long-lived-services` | [owner](../../R1/runner.md#long-lived-services) |
| `docs/08-runner-role.md#what-an-instance-is` | [owner](../../R1/runner.md#what-an-instance-is) |
| `docs/08-runner-role.md#who-it-runs-as` | [owner](../../R1/runner.md#who-it-runs-as) |
| `docs/08-runner-role.md#fits-the-other-pieces` | [owner](../../R1/runner.md#fits-the-other-pieces) |
| `docs/05-discovery.md#where-a-member-says-it-is` | [owner](../../R1/discovery.md#where-a-member-says-it-is) |
| `docs/05-discovery.md#health-checker` | [owner](../../R1/discovery.md#health-checker) |
| `docs/05-discovery.md#stats` | [owner](../../R1/discovery.md#stats) |
| `docs/05-discovery.md#exports` | [owner](../../R1/discovery.md#exports) |
| `docs/09-setup.md#reload` | [owner](../../R1/operations.md#reload) |
| `docs/03-services-and-topics.md#how-long-a-record-lives` | [owner](../../R1.1/records.md#how-long-a-record-lives) |
| `docs/02-access.md#service-to-service` | [owner](../../R1.1/access.md#service-to-service) |
| `docs/01-identity.md#how-to-reach-a-person` | [owner](../../R1.1/people.md#how-to-reach-a-person) |
| `docs/12-stages.md#the-image` | [owner](../../R1.1/README.md#scope) |
| `docs/future/1.2-UNDECIDED.md#shared-secrets-and-a-kv-with-locks` | [owner](../../R1.2/exploration.md#shared-secrets-and-a-kv-with-locks) |
| `docs/future/1.2-UNDECIDED.md#whether-the-daemons-own-parts-become-services` | [owner](../../R1.2/exploration.md#whether-the-daemons-own-parts-become-services) |
| `docs/future/1.2-UNDECIDED.md#a-public-directory-of-people-and-their-keys` | [owner](../../Future/public-directory.md#a-public-directory-of-people-and-their-keys) |
| `docs/09-setup.md#storage` | [owner](../../Future/storage.md#storage) |
| `docs/08-runner-role.md#in-process-queue` | [owner](../../Future/local-queues.md#in-process-queue) |
| `docs/05-discovery.md#debug-mode` | [owner](../../Future/debug.md#debug-mode) |
| `docs/10-modules.md#modules` | [owner](../../R1/modules.md#modules) |
| `docs/10-modules.md#the-hot-path` | [owner](../../R1/modules.md#the-hot-path) |
| `docs/10-modules.md#what-this-buys` | [owner](../../R1/modules.md#what-this-buys) |
| `docs/12-stages.md#poc` | [owner](../../done/PoC/scope.md#poc) |
| `docs/12-stages.md#r1` | [owner](../../R1/README.md#scope) |
| `docs/12-stages.md#r11` | [owner](../../R1.1/README.md#scope) |
| `Plans/MVP/TODO.md#d--the-bus-stops-reading-payloads` | [owner](../../R1/encryption-wave.md#d--the-bus-stops-reading-payloads) |

## Decision mapping

D identifiers are the settled-row ordinal in the [original index](decisions-before-rewrite.md#settled). A destination records substance, pending work or explicitly superseded history; migration is not a fresh design approval.

| Original row | Destination |
|---|---|
| D1 | [owner](../../../CLAUDE.md#versioning) |
| D2 | [owner](../../../docs/09-setup.md#build-information) |
| D3 | [owner](../../../docs/11-processes.md#process-titles) |
| D4 | [owner](../../../docs/01-identity.md#names) |
| D5 | [owner](../../../docs/01-identity.md#names) |
| D6 | [owner](../../R1/identity.md#sigils) |
| D7 | [owner](../../R1/identity.md#sigils) |
| D8 | [owner](../../../docs/09-setup.md#the-two-accounts) |
| D9 | [owner](../../../docs/09-setup.md#the-programs) |
| D10 | [owner](../../../docs/09-setup.md#the-programs) |
| D11 | [owner](../../../docs/02-access.md#getting-a-token) |
| D12 | [owner](../../../docs/01-identity.md#proving-possession) |
| D13 | [owner](../../../docs/09-setup.md#the-programs) |
| D14 | [owner](../../../docs/09-setup.md#the-two-accounts) |
| D15 | [owner](../../../docs/01-identity.md#acl) |
| D16 | [owner](../../../docs/01-identity.md#registration) |
| D17 | [owner](../../../docs/01-identity.md#registration) |
| D18 | [owner](../../R1/access.md#enrolment-policy) |
| D19 | [owner](../../../docs/01-identity.md#profile-fields) |
| D20 | [owner](../../../docs/01-identity.md#who-may-write-a-record) |
| D21 | [owner](../../../docs/01-identity.md#who-may-write-a-record) |
| D22 | [owner](../../../docs/01-identity.md#every-identifying-field-is-unique) |
| D23 | [owner](../../R1.1/people.md#how-to-reach-a-person) |
| D24 | [owner](../../Future/FUTURE.md#candidates) |
| D25 | [owner](../../../docs/01-identity.md#acl) |
| D26 | [owner](decisions-before-rewrite.md#superseded) |
| D27 | [owner](../../R1/identity.md#sigils) |
| D28 | [owner](../../R1/identity.md#sigils) |
| D29 | [owner](../../R1/identity.md#sigils) |
| D30 | [owner](../../R1/identity.md#sigils) |
| D31 | [owner](../../../docs/05-discovery.md#audience) |
| D32 | [owner](../../../docs/02-access.md#encrypted-sessions) |
| D33 | [owner](../../../docs/01-identity.md#proving-possession) |
| D34 | [owner](../../../docs/01-identity.md#proving-possession) |
| D35 | [owner](../../../docs/01-identity.md#proving-possession) |
| D36 | [owner](../../../docs/01-identity.md#acl) |
| D37 | [owner](../../R1/identity.md#delegation) |
| D38 | [owner](../../R1/identity.md#ownership) |
| D39 | [owner](../../../docs/01-identity.md#ownership) |
| D40 | [owner](../../R1/identity.md#ownership) |
| D41 | [owner](../../R1/identity.md#sealed-private-config) |
| D42 | [owner](../../../docs/02-access.md#what-a-call-carries) |
| D43 | [owner](../../../docs/02-access.md#getting-a-token) |
| D44 | [owner](../../../docs/02-access.md#getting-a-token) |
| D45 | [owner](../../../docs/02-access.md#token-scope) |
| D46 | [owner](../../R1/access.md#token-scope) |
| D47 | [owner](../../../docs/02-access.md#getting-a-token) |
| D48 | [owner](../../../docs/02-access.md#local-socket) |
| D49 | [owner](../../../docs/02-access.md#token-lifetime) |
| D50 | [owner](../../../docs/02-access.md#token-lifetime) |
| D51 | [owner](../../../docs/02-access.md#token-lifetime) |
| D52 | [owner](../../../docs/01-identity.md#ownership) |
| D53 | [owner](../../../docs/09-setup.md#storage) |
| D54 | [owner](../../../docs/04-messaging.md#durability) |
| D55 | [owner](../../../docs/04-messaging.md#durability) |
| D56 | [owner](../../../docs/02-access.md#local-socket) |
| D57 | [owner](../../../docs/02-access.md#local-socket) |
| D58 | [owner](../../R1/access.md#key-modes) |
| D59 | [owner](../../R1/access.md#encrypted-sessions) |
| D60 | [owner](../../R1/access.md#encrypted-sessions) |
| D61 | [owner](../../R1/runner.md#additional-script-forms) |
| D62 | [owner](../../R1.1/access.md#service-to-service) |
| D63 | [owner](../../R1/access.md#key-confirmation) |
| D64 | [owner](../../R1/locks.md#shared-locks) |
| D65 | [owner](../../R1/locks.md#a-set-of-locks) |
| D66 | [owner](../../R1/locks.md#a-set-of-locks) |
| D67 | [owner](../../../docs/01-identity.md#names) |
| D68 | [owner](../../../docs/01-identity.md#names) |
| D69 | [owner](../../R1/README.md#scope) |
| D70 | [owner](../../../docs/03-services-and-topics.md#service-and-template) |
| D71 | [owner](../../../docs/03-services-and-topics.md#service-and-template) |
| D72 | [owner](../../../docs/03-services-and-topics.md#configuring-a-template) |
| D73 | [owner](../../../docs/03-services-and-topics.md#configuring-a-template) |
| D74 | [owner](../../../docs/03-services-and-topics.md#configuring-a-template) |
| D75 | [owner](../../../docs/03-services-and-topics.md#why-a-digest-at-all) |
| D76 | [owner](../../../docs/03-services-and-topics.md#service-and-template) |
| D77 | [owner](../../../docs/03-services-and-topics.md#service-and-template) |
| D78 | [owner](../../../docs/03-services-and-topics.md#topics) |
| D79 | [owner](../../R1/registry.md#registry-sync) |
| D80 | [owner](../../R1/federation.md#chaining) |
| D81 | [owner](../../../docs/04-messaging.md#inbox-queues) |
| D82 | [owner](../../../docs/04-messaging.md#verbs) |
| D83 | [owner](../../../docs/04-messaging.md#message-fields) |
| D84 | [owner](../../../docs/04-messaging.md#receipts) |
| D85 | [owner](../../../docs/04-messaging.md#message-ttl) |
| D86 | [owner](../../../docs/04-messaging.md#push-and-pull) |
| D87 | [owner](../../../docs/04-messaging.md#overflow) |
| D88 | [owner](../../Future/storage.md#storage) |
| D89 | [owner](../../../docs/04-messaging.md#envelope) |
| D90 | [owner](../../../docs/05-discovery.md#what-a-listing-answers) |
| D91 | [owner](../../../docs/05-discovery.md#what-a-listing-answers) |
| D92 | [owner](../../../docs/05-discovery.md#what-a-listing-answers) |
| D93 | [owner](../../../docs/05-discovery.md#what-a-listing-answers) |
| D94 | [owner](../../../docs/05-discovery.md#what-it-shows) |
| D95 | [owner](../../../docs/05-discovery.md#refusals) |
| D96 | [owner](../../../docs/05-discovery.md#dashboard) |
| D97 | [owner](../../../docs/05-discovery.md#dashboard) |
| D98 | [owner](../../../docs/05-discovery.md#dashboard) |
| D99 | [owner](../../../docs/05-discovery.md#where-it-listens) |
| D100 | [owner](../../Future/debug.md#debug-mode) |
| D101 | [owner](../../../docs/11-processes.md#what-is-shared) |
| D102 | [owner](../../../docs/11-processes.md#how-a-child-is-started) |
| D103 | [owner](../../R1/auth.md#where-it-runs) |
| D104 | [owner](../../R1/auth.md#topology) |
| D105 | [owner](../../R1/auth.md#where-it-runs) |
| D106 | [owner](../../R1/auth.md#ssh-admin) |
| D107 | [owner](../../Future/billing.md#billing-role--future) |
| D108 | [owner](../../Future/billing.md#billing-role--future) |
| D109 | [owner](../../../docs/08-runner-role.md#adapters) |
| D110 | [owner](../../../docs/12-stages.md#stages) |
| D111 | [owner](../../done/PoC/scope.md#poc) |
| D112 | [owner](../../../docs/04-messaging.md#request-and-reply) |
| D113 | [owner](../../done/PoC/scope.md#poc) |
| D114 | [owner](../../done/PoC/scope.md#poc) |
| D115 | [owner](../../done/PoC/scope.md#poc) |
| D116 | [owner](../../../docs/01-identity.md#names) |
| D117 | [owner](../../../docs/08-runner-role.md#script-services) |
| D118 | [owner](../../R1/runner.md#additional-script-forms) |
| D119 | [owner](../../R1/runner.md#additional-script-forms) |
| D120 | [owner](../../R1/runner.md#long-lived-services) |
| D121 | [owner](../../R1/runner.md#additional-script-forms) |
| D122 | [owner](../../R1/runner.md#long-lived-services) |
| D123 | [owner](../../R1/runner.md#one-name-on-many-hosts) |
| D124 | [owner](../../R1/runner.md#one-name-on-many-hosts) |
| D125 | [owner](../../R1/runner.md#one-name-on-many-hosts) |
| D126 | [owner](../../../docs/01-identity.md#names) |
| D127 | [owner](../../../docs/01-identity.md#names) |
| D128 | [owner](../../R1/runner.md#one-name-on-many-hosts) |
| D129 | [owner](../../R1/runner.md#what-an-instance-is) |
| D130 | [owner](../../R1.1/records.md#how-long-a-record-lives) |
| D131 | [owner](../../R1.1/records.md#how-long-a-record-lives) |
| D132 | [owner](../../R1.1/records.md#how-long-a-record-lives) |
| D133 | [owner](../../R1/identity.md#ownership) |
| D134 | [owner](../../R1/identity.md#ownership) |
| D135 | [owner](../../R1/identity.md#ownership) |
| D136 | [owner](../../R1/runner.md#backing-it-up) |
| D137 | [owner](../../R1/runner.md#backing-it-up) |
| D138 | [owner](../../R1/runner.md#the-list-of-what-is-installed) |
| D139 | [owner](../../R1/runner.md#the-list-of-what-is-installed) |
| D140 | [owner](../../R1/runner.md#the-list-of-what-is-installed) |
| D141 | [owner](../../R1/runner.md#what-it-comes-after) |
| D142 | [owner](../../R1/runner.md#what-it-comes-after) |
| D143 | [owner](../../R1/runner.md#what-it-comes-after) |
| D144 | [owner](../../R1/runner.md#backing-it-up) |
| D145 | [owner](../../R1.1/services.md#rules-they-all-obey) |
| D146 | [owner](../../R1.1/services.md#rules-they-all-obey) |
| D147 | [owner](../../R1.1/services.md#rules-they-all-obey) |
| D148 | [owner](../../R1.1/services.md#for-the-agents-themselves) |
| D149 | [owner](../../R1.1/services.md#for-the-agents-themselves) |
| D150 | [owner](../../R1.1/services.md#for-the-agents-themselves) |
| D151 | [owner](../../R1.1/services.md#one-contract-for-the-set) |
| D152 | [owner](../../R1.1/services.md#one-contract-for-the-set) |
| D153 | [owner](../../R1.1/services.md#one-contract-for-the-set) |
| D154 | [owner](../../R1.1/services.md#rules-they-all-obey) |
| D155 | [owner](../../R1.1/services.md#the-bus-watching-itself) |
| D156 | [owner](../../R1.1/services.md#the-bus-watching-itself) |
| D157 | [owner](../../R1.1/services.md#people-and-the-world-outside) |
| D158 | [owner](../../R1.1/services.md#people-and-the-world-outside) |
| D159 | [owner](../../R1.1/people.md#how-to-reach-a-person) |
| D160 | [owner](../../R1.1/services.md#data) |
| D161 | [owner](../../R1.1/services.md#data) |
| D162 | [owner](../../R1.1/services.md#data) |
| D163 | [owner](../../R1.1/README.md#scope) |
| D164 | [owner](../../R1.1/README.md#scope) |
| D165 | [owner](../../R1.1/services.md#reading-the-box) |
| D166 | [owner](../../R1.1/services.md#other-buses) |
| D167 | [owner](../../R1.1/services.md#people-and-the-world-outside) |
| D168 | [owner](../../done/PoC/scope.md#poc) |
| D169 | [owner](../../../docs/04-messaging.md#one-reader-per-inbox) |
| D170 | [owner](../../../docs/04-messaging.md#reply-routing) |
| D171 | [owner](../../Future/billing.md#billing-role--future) |
| D172 | [owner](../../../docs/04-messaging.md#one-reader-per-inbox) |
| D173 | [owner](../../done/PoC/scope.md#poc) |
| D174 | [owner](../../done/PoC/scope.md#poc) |
| D175 | [owner](../../done/PoC/scope.md#poc) |
| D176 | [owner](../../../docs/04-messaging.md#push-and-pull) |
| D177 | [owner](../../../docs/08-runner-role.md#adapters) |
| D178 | [owner](../../R1/modules.md#modules) |
| D179 | [owner](../../../docs/10-modules.md#languages) |
| D180 | [owner](../../../docs/10-modules.md#external-tools) |
| D181 | [owner](../../../docs/10-modules.md#external-tools) |
| D182 | [owner](../../../docs/10-modules.md#external-tools) |
| D183 | [owner](../../../docs/10-modules.md#external-tools) |
| D184 | [owner](../../../docs/03-services-and-topics.md#configuring-a-template) |
| D185 | [owner](../../../docs/04-messaging.md#inbox-queues) |
| D186 | [owner](../../../docs/03-services-and-topics.md#how-to-call-it) |
| D187 | [owner](../../../docs/03-services-and-topics.md#how-to-call-it) |
| D188 | [owner](../../../docs/05-discovery.md#what-a-listing-answers) |
| D189 | [owner](../../../docs/10-modules.md#the-rule) |
| D190 | [owner](../../../docs/11-processes.md#the-rule) |
| D191 | [owner](../../../docs/11-processes.md#the-processes) |
| D192 | [owner](../../../docs/11-processes.md#what-is-shared) |
| D193 | [owner](../../../docs/11-processes.md#the-rule) |
| D194 | [owner](../../../docs/11-processes.md#nothing-the-daemon-runs-may-exec) |
| D195 | [owner](../../../docs/09-setup.md#the-two-accounts) |
| D196 | [owner](../../../docs/02-access.md#the-three-doors) |
| D197 | [owner](../../R1/runner.md#reaching-the-runner) |
| D198 | [owner](../../R1/runner.md#reaching-the-runner) |
| D199 | [owner](../../R1/operations.md#runner-unit) |
| D200 | [owner](../../R1/operations.md#runner-unit) |
| D201 | [owner](../../R1/operations.md#runner-unit) |
| D202 | [owner](../../R1/runner.md#what-an-instance-is) |
| D203 | [owner](../../R1/runner.md#the-three-env-layers) |
| D204 | [owner](../../R1/runner.md#the-three-env-layers) |
| D205 | [owner](../../R1/runner.md#what-the-child-is-told) |
| D206 | [owner](../../R1/runner.md#what-the-child-is-told) |
| D207 | [owner](../../R1/runner.md#what-an-instance-is) |
| D208 | [owner](../../R1/runner.md#reaching-the-runner) |
| D209 | [owner](../../R1/runner.md#where-it-runs) |
| D210 | [owner](../../R1/runner.md#what-an-instance-is) |
| D211 | [owner](../../R1/runner.md#what-the-runner-does) |
| D212 | [owner](../../R1/runner.md#what-the-runner-does) |
| D213 | [owner](../../R1/runner.md#who-it-runs-as) |
| D214 | [owner](../../../docs/11-processes.md#why-the-supervisor-holds-cap_chown) |
| D215 | [owner](../../../docs/10-modules.md#the-rule) |
| D216 | [owner](../../../docs/10-modules.md#external-tools) |
| D217 | [owner](../../../docs/10-modules.md#the-rule) |
| D218 | [owner](../../../docs/00-overview.md#goal) |
| D219 | [owner](../../Future/FUTURE.md#candidates) |
| D220 | [owner](../../../docs/04-messaging.md#one-reader-per-inbox) |
| D221 | [owner](../../../docs/04-messaging.md#one-reader-per-inbox) |
| D222 | [owner](../../../docs/04-messaging.md#one-reader-per-inbox) |
| D223 | [owner](../../../docs/08-runner-role.md#sandboxing) |
| D224 | [owner](../../../docs/08-runner-role.md#sandboxing) |
| D225 | [owner](../../../docs/08-runner-role.md#stopping-it-and-reading-what-it-said) |
| D226 | [owner](../../../docs/08-runner-role.md#stopping-it-and-reading-what-it-said) |
| D227 | [owner](../../../docs/04-messaging.md#request-and-reply) |
| D228 | [owner](../../../docs/04-messaging.md#several-readers-may-wait-when-they-say-so) |
| D229 | [owner](../../../docs/04-messaging.md#receipts) |
| D230 | [owner](../../../docs/04-messaging.md#receipts) |
| D231 | [owner](../../../docs/04-messaging.md#inbox-queues) |
| D232 | [owner](../../../docs/04-messaging.md#subscribers) |
| D233 | [owner](../../../docs/04-messaging.md#subscribers) |
| D234 | [owner](../../../docs/04-messaging.md#subscribers) |
| D235 | [owner](../../../docs/04-messaging.md#overflow) |
| D236 | [owner](../../../docs/04-messaging.md#request-and-reply) |
| D237 | [owner](../../../docs/04-messaging.md#verbs) |
| D238 | [owner](../../../docs/04-messaging.md#request-and-reply) |
| D239 | [owner](../../../docs/04-messaging.md#request-and-reply) |
| D240 | [owner](../../../docs/04-messaging.md#request-and-reply) |
| D241 | [owner](../../../docs/05-discovery.md#rules-it-is-built-to) |
| D242 | [owner](../../../docs/05-discovery.md#rules-it-is-built-to) |
| D243 | [owner](../../R1/discovery.md#dashboard-extensions) |
| D244 | [owner](../../../docs/05-discovery.md#rules-it-is-built-to) |
| D245 | [owner](../../../docs/05-discovery.md#signing-in) |
| D246 | [owner](../../../docs/05-discovery.md#signing-in) |
| D247 | [owner](../../../docs/05-discovery.md#what-it-shows) |
| D248 | [owner](../../../docs/01-identity.md#profile-fields) |
| D249 | [owner](../../R1/identity.md#groups-and-roles) |
| D250 | [owner](../../../docs/08-runner-role.md#script-services) |
| D251 | [owner](../../../docs/08-runner-role.md#script-services) |
| D252 | [owner](../../../docs/08-runner-role.md#script-services) |
| D253 | [owner](../../../docs/04-messaging.md#receipts) |
| D254 | [owner](../../../docs/08-runner-role.md#script-services) |
| D255 | [owner](../../../docs/04-messaging.md#message-ttl) |

## Questions

Q1–Q28 retain the ordinal of the [original open rows](decisions-before-rewrite.md#open). New questions preserve previously unindexed gaps and contradictions. Their current homes are the QUESTIONS files in [the plan index](../../README.md#stages).

## Validation

Internal Markdown paths and anchors checked across the current docs, plans,
archives and source READMEs; a scratch broken-link control was rejected.
Original open-question IDs each have one owning plan, and the displaced encryption
wave retains its task IDs. No removed doc paths remain in live docs or source
references. Whitespace checks passed. Runtime behavior was not changed or retested
by this documentation migration; earlier program-version evidence is separate.

The owner’s subsequent R1 backup-encryption decision is recorded in
[R1 decisions](../../R1/DECISIONS.md#backup-encryption-choice).
