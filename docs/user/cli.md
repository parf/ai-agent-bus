# Command line: use the bus from a terminal

📌 **TL;DR:**
- **Problem:** connecting a script, a person and an AI usually takes a different tool, login and password for each.
- **Outcome:** one command, `agent-bus`, registers names, sends and reads messages, asks agents for help and runs a script as an agent.
- **Start:** check that the bus is up and knows you: `agent-bus status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Avoid another password.** A configured local account socket authenticates you without copying a token.
- 📡 **Ten servers, ten terminals.** Now one `call --tagged` asks every matching agent at once and lists the answers.
- 🏢 **It grows with you:** the same commands work on one laptop and across many hosts, people and teams.

## What you get

- ✅ **Talk:** `send`, `call` (send and wait), `consume` (read your inbox), `reply`, and `publish` to many.
- ✅ **Discover:** `ls` lists who can take a message now; `help '#agent'` says what an agent can do.
- ✅ **Describe things:** `register`, `manage` and `unregister` names, and set who may use them.
- ✅ **Run a script as an agent:** `agent-bus run`, in the foreground, with [optional confinement](../runner/security.md#isolation-today).
- ✅ **Share state:** stored values and named locks on a record you manage.
- ⚠️ **Agent names begin with `#`:** quote them (`'#echo@demo'`).
- 🚫 **Not for:** installing the node (`agent-bus-setup`) or adding people (`agent-bus-admin`); see [the tools](#-the-command-line-tools).

## Try this first

- Ask the bus how it is and who it thinks you are:

```sh
agent-bus status
```

- Check the `you` field against the identity you intend to use. Other fields describe the bus; see [health counters](daemon.md#is-it-healthy).
- Somebody must have installed it already ([setup](../07-setup.md#install)).

See the available commands, then list visible agents with waiting readers:

```sh
agent-bus --help
agent-bus ls --table
```

<details>
<summary>Command help, shortened</summary>

```text
USAGE
  agent-bus [--addr=<address>] <command> [<args>]
  agent-bus <command> --help · help '#agent' · --version

DESCRIPTION
  Talk to the bus: send messages, list records and run agents.

MAIN COMMANDS
  status                          Show bus status and your name
  ls [-h] [--all] [<name>]        List visible records (readers by default)
  send <to> <text>                Send once; do not wait
  call <to> <text>                Send once and wait for answer
...
```

</details>

Use `agent-bus ls --all` for the complete visible registry, including records without a waiting reader. Examples below use illustrative names such as `#echo@demo`; create them as shown or substitute names you manage.

## 🧰 The command-line tools

Five programs; you use the first one almost always.

| Program | It is for | You are |
|---|---|---|
| **`agent-bus`** | 💬 everyday use: register, send, read, run an agent, get your credential | anyone |
| **`agent-bus-admin`** | 👤 adding people, handing out credentials, backups ([below](#-for-the-bus-operator)) | the bus operator |
| **`agent-bus-setup`** | 📦 installing the node once ([setup](../07-setup.md#install)) | root, once |
| **`agent-bus-runner`** | 🏃 installed scripts, kept running without a terminal ([runner guide](../runner/README.md)) | the runner account |
| **`agent-busd`** | ⚙️ the bus itself ([the daemon](daemon.md)) | it runs itself |

## Help

- `agent-bus --help` or `agent-bus -h` groups every command by purpose.
- `agent-bus <command> --help` or `<command> -h` shows its usage, options with defaults, and examples. The listings are the exception: in `ls` and `bigfile ls`, `-h` is the human form, as in Unix `ls -h`. Local help answers even when a launcher session file is no longer readable.
- `agent-bus help '#agent'` asks an agent instead ([below](#asking-an-agent-what-it-can-do)).
- `agent-bus ls` displays a table on a terminal and JSON when piped or redirected. `-h` or `--table` and `--json` force either form.
- `agent-bus bigfile ls <record> -h` adds a header and sizes like `1.5K`; without it, lines are tab-separated with sizes in bytes.
- `--addr=<address>` goes before the command.

## Option spelling

- Write long options as `--name` or `--name=value`.
- A single dash is for numeric concurrency options such as `-4` for `run`.
- Single-dash long spellings are also accepted; use the documented `--` form in new commands.

## 🔌 How it finds the bus, and how it knows you

You usually do not have to think about either.

| Where you are | What to do |
|---|---|
| **on your own machine** | after your account is mapped to a bus User, `agent-bus` discovers its private socket; **no token is needed** |
| **somewhere else** | set `AGENT_BUS_ADDR` to the bus address and `AGENT_BUS_TOKEN` to your credential; use HTTPS for an untrusted network, or forward your socket over SSH ([remote access](../02-access-remote.md#choosing-a-way)) |
| **over TLS** | `AGENT_BUS_ADDR=https://host:port`, and `AGENT_BUS_TLS_FINGERPRINT` to what `ssh agent-busd@thehost token --fingerprint` prints ([TLS](../02-access-remote.md#over-https)) |
| **a specific bus** | `--addr=<socket-path>` or `--addr=http://host:port` beats both |
| **inside an `ab-*` launcher session** | nothing: it acts as the session's current name, from the launcher's session file; `env -u AGENT_BUS_SESSION_FILE agent-bus …` for a one-off as another identity ([launchers](../06-launchers.md#the-sessions-shell-after-a-rename)) |

- ⚠️ **`set AGENT_BUS_TOKEN` as an answer** means you reached the shared socket, not your own.
- Ask the bus operator to add you, or set a token.

Get your credential:

- **Over SSH:** `export AGENT_BUS_TOKEN=$(ssh agent-busd@thehost token)`
- **On the box, as you:** `agent-bus token`
- **With a key, no SSH:** `agent-bus token me@myhost --key="$HOME/.ssh/id_ed25519"`
- 💡 Asking twice gives the **same** token: it is a read, not a reset.
- `--rotate` replaces it; the old one keeps working until the next rotation, so nobody is stranded.

## 📇 Registering things

- **Registering states a description.** The thing described does not have to know the bus exists.
- A name with `#` is an agent; a name without `#` and without `--kind` is a 📡 service, which needs `--service-addr` and `--protocol`.
- **Register an agent:** `agent-bus register '#echo@demo' --descr "says it back"`
- **See where a pool's members are:** `agent-bus ls --table '#name@realm'` adds a `Hosts` line, such as `srv1 (now), srv2 (40s)`.
- **Remove the entry:** `agent-bus unregister '#echo@demo'`
- ℹ️ `unregister` removes the entry, not the process; something still serving the name registers it again.

What `ls` shows:

- Plain `ls` lists only agents being read right now; `--kind=agent` lists every agent, `--all` everything you may see.
- `--tag=<tag>` (repeatable or comma-separated) keeps records carrying every given tag.
- `READERS`: reads waiting right now. `QUEUED`: messages waiting for them. `LAST USED`: when the name's credential last made a call.
- 🔍 `READERS 0` means no read is waiting at that instant. With a growing queue, check whether the worker is busy, disconnected or stopped; the counter alone cannot distinguish them.
- A 📡 service and a 👥 group show `-`: a service is [something outside](../03-records-service.md#what-a-service-is), a group is a list of members.

Useful flags when you register:

| Flag | |
|---|---|
| `--descr` | what it is, in a few words; what everyone else sees |
| `--allow=a@b,c@d` or `--allow='*'` | who may use it; leave it out and the bus decides |
| `--ttl=1h` · `--bound=1000` | how long an agent's or queue's inbox keeps things, and how many; a 📣 pubsub takes neither |
| `--overflow=ring\|strict` | when full: drop the oldest, or refuse new ones |
| `--kind=queue\|pubsub` | a channel instead of an agent ([below](#-channels-for-one-to-many)) |
| `--service-addr` · `--protocol` | where a 📡 [service](../03-records-service.md#how-to-call-it) really lives |

### 🔐 The credential for something outside

A 📡 [service](../03-records-service.md#what-a-service-is) usually needs a password. Keep it on the record:

- Register the service first with its address and protocol, as described in the [service guide](../03-records-service.md#how-to-call-it).
- **Set it:** `agent-bus secret db@demo 'PGPASSWORD=s3cret'`
- **From a file:** `agent-bus secret db@demo - < .env`
- **Read it back:** `password=$(agent-bus secret db@demo)`
- Secret uses the [accepted environment-file format](../runner/configuration.md#environment-files); a shell command substitution removes trailing newlines, so use file redirection when you need the exact bytes.
- 🔍 The record's Owner and Maintainers may read it, and an Agent its own; nobody else, and group membership alone grants nothing ([private values](../constitution.md#-private-values)).

### ✏️ Changing a record

- These examples use an existing `jobs@demo` record and existing members you manage.
- **Add someone unless already there:** `agent-bus manage jobs@demo --add-to-set-allow bob@demo`
- **Take them out:** `agent-bus manage jobs@demo --remove-allow bob@demo`
- **Route a queue to an agent:** `agent-bus manage jobs@demo --deliver-to '#worker@demo'`
- **Hide it:** `agent-bus manage jobs@demo --status inactive`; `--status active` brings it back.
- **Replace the labels:** `agent-bus manage jobs@demo --tags 'work,alerts'`; `--tags=` clears them ([tag rules](../03-records.md#tags)).
- **Set a group's whole membership:** `agent-bus group @crew alice@demo '#worker@demo'`
- Every list (allow, maintainers, deliver-to, tags) takes `--add-`, `--add-to-set-` and `--remove-`; each incremental list edit is atomic, avoiding a read-modify-replace race.
- An inactive record is absent from ordinary listings and operations until reactivated; authorized callers can inspect the separate [inactive view](../09-daemon-api.md#registry).

## ✉️ Sending and receiving

Send a request, wait for its answer, or read your own inbox:

- **Send, no answer wanted:** `agent-bus send '#echo@demo' "no answer wanted"`
- **Call and wait for the reply:** `agent-bus call '#hi@demo' --wait 10s "ping"`
- **Read your own inbox:** `agent-bus consume --wait 30s`; add `--follow` to keep reading.
- A `call` writes receipts to stderr and the answer envelope to stdout.
- `call <to> <text> --body` prints only the answer body, followed by a newline, like `call <to> <text> | jq -r .body`.
- `--body` is for a single target, without `--tagged`. A receipt-only or refused call exits nonzero, writes its reason to stderr, and prints no answer on stdout.

| Word | Means |
|---|---|
| **`ack`** | it arrived and something has it; **not** that the work started |
| **`done`** | the work finished, with nothing to hand back |
| **a reply** | the work finished, and here is the answer |

Answering by hand:

- `agent-bus ack <message-id>`, `agent-bus done <message-id>`, `agent-bus reply <message-id> "here you go"`
- **Unprompted:** `agent-bus reply --to '#hi@demo' --topic t "unprompted"`
- **Answer to another inbox:** `send --reply-to=<name>` keeps the message's `--topic` and `--tag` for matching; the destination must admit the answering agent ([reply routes](../04-messaging.md#request-and-reply)).
- ⚠️ **One reader per inbox.** A second reader is refused; `consume --share` asks for several workers on purpose. `--host=name` identifies a reader's host in a pool listing.

### 📣 Channels, for one-to-many

Create a receiver and channel you own, admit the channel at the receiver, then configure delivery:

```sh
agent-bus register alerts@demo --kind pubsub --descr "alerts"
agent-bus register '#alert-inbox@demo' --allow alerts@demo
agent-bus manage alerts@demo --deliver-to '#alert-inbox@demo'
agent-bus publish --channel alerts@demo "disk is filling up"
```

The message waits in `#alert-inbox@demo`; start its reader to process it. An empty pubsub Deliver-To list is refused. A recipient can leave with `agent-bus unsubscribe alerts@demo` when authenticated as that recipient.

- ⚠️ **The owner chooses who receives.** You cannot add yourself; `unsubscribe` takes you off, because it is your inbox that fills.
- ⚠️ **A channel is not a topic.** The channel is the name you publish to; `--topic` labels one message ([channels](../03-records-channel.md#channel-not-topic)).

| Kind | Goes to |
|---|---|
| `queue` | **one** consumer, and waits until somebody takes it |
| `pubsub` | **everybody on its [Deliver-To list](../04-messaging.md#deliver-to)**, kept for nobody |

## Checking whether you may send

Choose a target from your listing and check your own identity without sending:

```sh
agent-bus may-send '#target'
```

A successful response includes `may_send: true`. A refusal prints its reason on stderr and exits nonzero. The check reserves nothing; [permission rules](../09-daemon-api.md#checking-permission-to-send) also apply when you send.

## Asking an agent what it can do

Replace `#target` with the name of a running agent from your listing:

```sh
agent-bus help '#target'
```

- Help describes the methods exposed by that agent; native help may filter them by the caller's roles or the instance's configured scope.
- **Configuration:** `agent-bus help '#target' config`.
- **Secret key names and instructions, never values:** `agent-bus help '#target' secret`.
- **The complete description:** `agent-bus help '#target' json`.
- Terminal output is coloured; pipes and files get plain text. `NO_COLOR` or `--no-color` turns colour off.
- A refusal or timeout goes to stderr and exits nonzero; `--wait 5s` shortens the 30-second default.

- To receive the original message, use `agent-bus call '#target' --help-json`; its body contains the description.
- Other asks are `--help`, `--help-config`, `--help-secret`, `--status` and `--mcp`; use one at a time. `send` accepts them too.
- See [package help](../runner/configuration.md#package-help) for how agents answer.

## Calling many agents at once

Ask the same question of every agent with a tag:

```sh
agent-bus ls --tag systemd --table
```

Choose a tag present on agents you intend to call. For installed systemd agents, ask for failed units:

```sh
agent-bus call --tagged systemd --wait 10s failed
```

- ✅ Answers come back under name-sorted headings; `No answer:` and `Refused:` lists follow, one line per agent.
- ✅ Exit 0 means every selected agent answered.
- ⚠️ **One shared deadline:** `--wait` (30 seconds by default) covers all targets together.
- ⚠️ **Never retried:** a missing answer does not prove nothing happened; do not blindly resend.
- 🚫 Only agents you can see are called; other kinds, untagged and hidden agents never are.
- Quote a multi-word body: the tagged form takes exactly one.
- More rules: [technical details](#technical-details).

## ⚙️ Running a script as an agent

One command line, and your script is an agent:

- `agent-bus run '#hi@demo' /full/path/to/hi.sh --descr "greets"` serves `#hi@demo` with your script.
- `agent-bus call '#hi@demo' "ping"` then runs it; whatever it prints comes back as the answer.
- **Watch its output:** `agent-bus logs '#hi@demo' --lines 50 --follow`
- **Stop it:** `agent-bus stop '#hi@demo'`

- The default input is the body as `$1`; `--algo=json` passes the whole message on stdin.
- See [run options](dedicated-runner.md#run-options) for concurrency, access, metadata and confinement.

- ⚠️ **Use an absolute path.** The script runs in its own work folder, so a relative path is refused; a bare command on `PATH`, such as `echo`, is fine ([script limits](../06-dedicated-runner.md#script-agents)).
- 💡 **It runs in the foreground.** Ctrl-c stops it and, with nothing queued, unregisters the name.
- 🔁 A bus restart or dropped link does not end it: it waits and reconnects ([watching it](dedicated-runner.md#-watching-and-stopping-it)).
- An external supervisor that restarts it uses `--stay-registered` ([lifecycle](../06-dedicated-runner.md#keeping-a-supervised-registration)).
- To keep a script served without a terminal open, use the [runner](../runner/README.md).
- More: [running an agent](dedicated-runner.md).

## 🪪 Joining a bus you are new to

- If the bus trusts a directory that publishes your public key, such as GitHub, introduce yourself: `agent-bus enrol me@github --key="$HOME/.ssh/id_ed25519"`
- You need no `AGENT_BUS_TOKEN` for this: you have none yet. Point `AGENT_BUS_ADDR` at the bus ([above](#-how-it-finds-the-bus-and-how-it-knows-you)); every other command still asks for a credential.
- The bus sends a challenge, your key signs it, and you get an ordinary record.
- Fetching a public key proves nothing; **signing the challenge does**.

## Record storage and locks

For a record you may manage: stored values and named locks. All options and limits: [KV](../03-records-kv.md#key-value-store) and [locks](../01-identity-and-authority.md#shared-locks).

| Command | Result |
|---|---|
| `agent-bus kv get jobs@demo cursor --int` | read an integer |
| `agent-bus kv set jobs@demo cursor 42 --int` | store an integer |
| `agent-bus kv inc jobs@demo cursor` | add one atomically |
| `agent-bus kv delete jobs@demo cursor --int` | remove it |
| `agent-bus kv json jobs@demo batch '[{"op":"push","key":"todo","value":"job-1"}]'` | edit a JSON value atomically |
| `agent-bus bigfile put jobs@demo report ./report.pdf --ttl=2h` | store a value too big for a message (up to 32 MiB; default 24h, at most 30d; `--memory` keeps it in daemon memory) |
| `agent-bus bigfile get jobs@demo report -o ./copy.pdf` | download it, checked against its SHA-256; without `-o` it goes to stdout |
| `agent-bus bigfile ls jobs@demo` | list its bigfiles with size, SHA-256, where held and time left |
| `agent-bus bigfile rm jobs@demo report` | delete one now |
| `agent-bus lock jobs@demo deploy --ttl=30s --wait=10s` | wait for a named lock |
| `agent-bus try-lock jobs@demo deploy --ttl=30s` | try immediately |
| `agent-bus extend jobs@demo deploy --ttl=30s` | renew your hold |
| `agent-bus holders jobs@demo` | list current holds |
| `agent-bus release jobs@demo deploy` | release your hold |

## Agent configuration

- Secrets go in Secret, settings in Configuration ([configuration](../03-records-agent.md#configuring-a-template)).
- **Read:** `agent-bus config <name>` prints its JSON settings; an Agent may read its own.
- **Replace:** add JSON, or `-` for stdin, as Owner or Maintainer.
- Configuration is not secret: authorized web pages show it; other callers get nothing.
- Scripts receive it as compact `AGENT_BUS_CONFIG` at their next start ([configuration](../03-records-agent.md#configuring-a-template)).

## 🆘 When something looks wrong

| You see | It usually means |
|---|---|
| `set AGENT_BUS_TOKEN` | you are on the shared socket, not your own; see [above](#-how-it-finds-the-bus-and-how-it-knows-you) |
| `no inbox for …: register it first` | you asked to `consume` from a name nobody registered; a User's own inbox always exists |
| `no answer within 30s` | no answer arrived by the deadline; the operation may have taken effect, so inspect the outcome before any retry |
| a name that "does not exist" | sending to an unregistered name is refused on purpose, so you learn now |
| `ls --kind=agent` shows `READERS 0` and `QUEUED` climbing | work is not being drained fast enough; check the worker's logs and connection |

- `agent-bus status` shows the counters worth watching: `dropped`, `expired` and `refused`.

## 👤 For the bus operator

Host administration uses the daemon account through the admin tool. Record export/import instead require the bus Owner's own local login; do not run them through an account handover.

- **Let somebody in:** `agent-bus-admin user add alice@demo ~/.ssh/id_ed25519.pub`
- **As a bus operator:** `agent-bus-admin user add alice@demo - --admin < key.pub`
- **Fill a name from a local account:** `agent-bus-admin user import-local alice@demo alice`
- **A keyless user on that account's socket:** `agent-bus-admin user add-local worker@myhost worker`
- **List and remove:** `agent-bus-admin user list`, `agent-bus-admin user remove alice@demo`
- **Rotate a credential:** `agent-bus-admin token alice@demo --rotate`
- **Back up and restore records:** `agent-bus-admin export --include=tag:ops --out=ops.tar.gz.age`, then `agent-bus-admin import ops.tar.gz.age --dry-run --keep-existing=all` ([how-to](backup.md)).
- 🔒 A key added this way reaches **one command and no shell**: `agent-bus token` for a person, this program for an operator.
- Installing: [setup](../07-setup.md#install). The bus itself: [the daemon](daemon.md). Live AI sessions: [Claude Code, Codex and opencode](ai-cli.md).

## Technical details

<details>
<summary>Exit codes and how flags are read</summary>

- Success exits 0; invalid input, a refusal, a transport failure or a deadline exits 1.
- Each command accepts only its own flags; an unknown or misplaced `--flag` is refused by name before anything is sent.
- Boolean switches never consume the following name or text.
- A valued flag without its value is refused; `--descr=` supplies an explicit empty value.
- A bare `--` ends options: everything after it is literal, for example `agent-bus send '#worker@demo' -- "--verbose please"`.
- The current spellings are `config`, `ls --table`, `register --service-addr=…` and `register --kind=queue|pubsub`.

</details>

<details>
<summary>Tagged calls</summary>

- `call --tagged=<tag> [--wait=D] <body>` selects caller-visible agents from the tag-filtered listing, including agents with no reader right now.
- Tags label records; they grant no access and do not prove an agent is healthy.
- `ls --tag=<tag>` lists every visible matching record, readers or not.
- The tagged form refuses an explicit target and every call flag except `--wait`.
- It is `--tagged` because `--tag` on a single call is the correlation value; `call <target> --topic t --tag g <body>` keeps that meaning.
- Each target gets a fresh topic and correlation tag; calls run concurrently.
- The [one-reader rule](../04-messaging.md#one-reader-per-inbox) applies: call as an identity whose inbox nobody else is draining.
- A no-answer line reads `no answer within 10s (the message may have been accepted; do not resend it)`.
- Nonzero exit: a no-answer, a refusal, a lookup failure, invalid arguments, or no matching agents.
- A `done` receipt lands in the no-answer list with its reason, without waiting out the deadline.
- Transport or read failures also land under no-answer: the outcome may be unknown.
- Answer bodies are opaque: an application's own JSON error is still an answer.

</details>
