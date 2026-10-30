# Command line: use the bus from a terminal

📌 **TL;DR:**
- **Problem:** connecting a script, a person and an AI usually takes a different tool, login and password for each.
- **Outcome:** one command, `agent-bus`, registers names, sends and reads messages, asks agents for help and runs a script as an agent.
- **Start:** check that the bus is up and knows you: `agent-bus status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Every tool wants its own password.** Now your own account is your credential on your machine; no token to copy.
- 🤷 **You cannot tell what an agent does.** Now you ask it, `agent-bus help '#agent'`, and read its answer as plain text.
- 🧩 **Wiring a script into a team takes code.** Now one `agent-bus run` line puts it behind a name others can call.
- 📡 **Ten servers, ten terminals.** Now one `call --tagged` asks every matching agent at once and lists the answers.
- 🏢 **It grows with you:** the same commands work on one laptop and across many hosts, people and teams.

## What you get

- ✅ **Talk:** `send`, `call` (send and wait), `consume` (read your inbox), `reply`, and `publish` to many.
- ✅ **Discover:** `ls` lists who can take a message now; `help '#agent'` says what an agent can do.
- ✅ **Describe things:** `register`, `manage` and `unregister` names, and set who may use them.
- ✅ **Run a script as an agent:** `agent-bus run`, in the foreground, with optional confinement.
- ✅ **Share state:** stored values and named locks on a record you manage.
- ⚠️ **Agent names begin with `#`:** quote them (`'#echo@demo'`) or write `--agent echo@demo`.
- ⚠️ **One reader per inbox:** a second `consume` on the same name is refused unless you ask for `--share`.
- 🚫 **Not for:** installing the node (`agent-bus-setup`) or adding people (`agent-bus-admin`); see [the tools](#-the-command-line-tools).

## Try this first

- Ask the bus how it is and who it thinks you are:

```sh
agent-bus status
```

```text
{"up":"6h43m35s","realm":"parf.us","records":126,...,"dropped":0,"expired":0,...,"you":"#ab-w2a",...}
```

- ✅ If `you` shows your name, setup is done. Nothing else to configure.
- Somebody must have installed it already ([setup](../07-setup.md#install)).

<details>
<summary>Your first five minutes (commands that ran today)</summary>

See every command, grouped by purpose:

```sh
agent-bus help
```

```text
USAGE
  agent-bus [--addr=<address>] <command> [<args>]
  agent-bus help <command> · help '#agent' · --version | -h, --help

DESCRIPTION
  Talk to the bus: send messages, list records and run agents.

MAIN COMMANDS
  status                          Show bus status and your name
  ls [-h] [--all] [<name>]        List visible records (readers by default)
  send <to> <text>                Send once; do not wait
  call <to> <text>                Send once and wait for answer
...
```

List the agents that can take a message now, as a table:

```sh
agent-bus ls --table
```

```text
NAME                                    KIND     OWNER  READERS  QUEUED  LAST USED  DESCRIPTION
#debug@parf.us                          👾 Agent  parf   1        0       just now   Check delivery and runner health; ...
#slack@parf.us                          👾 Agent  parf   1        0       just now   Post and read messages; route replies; ...
...
```

Check that you may send to an agent, without sending:

```sh
agent-bus may-send '#systemd@parf.us'
```

```text
{"may_send":true}
```

The output is from the development node on 2026-10-30; names and counts differ on yours.

</details>

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

- `agent-bus help` groups every command by purpose.
- `agent-bus help <command>` or `agent-bus <command> --help` shows its usage, options with defaults, and examples.
- `agent-bus help '#agent'` asks an agent instead ([below](#asking-an-agent-what-it-can-do)).
- `-h` or `--help` alone shows the overview; `ls -h` is the short form of `ls --table`.
- `--addr=<address>` goes before the command; `--agent=<name>` is shorthand for `'#name'`.

## Option spelling

- Write long options as `--name` or `--name=value`.
- A single dash is for one-letter options such as `-h`.
- Older single-dash long spellings are still accepted.

## 🔌 How it finds the bus, and how it knows you

You usually do not have to think about either.

| Where you are | What to do |
|---|---|
| **on your own machine** | nothing: `agent-bus` finds your own socket, and the system already knows your account, so **no token is needed** |
| **somewhere else** | set `AGENT_BUS_ADDR` to `http://host:port` and `AGENT_BUS_TOKEN` to your credential, or forward your socket over SSH ([remote access](../02-access-remote.md#choosing-a-way)) |
| **over TLS** | `AGENT_BUS_ADDR=https://host:port`, and `AGENT_BUS_TLS_FINGERPRINT` to what `ssh agent-busd@thehost token --fingerprint` prints ([TLS](../02-access-remote.md#over-https)) |
| **a specific bus** | `--addr=<socket-path>` or `--addr=http://host:port` beats both |

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
- **List everything you may see:** `agent-bus ls --table --all`
- **Remove the entry:** `agent-bus unregister '#echo@demo'`
- ℹ️ `unregister` removes the entry, not the process; something still serving the name registers it again.

What `ls` shows:

- Plain `ls` lists only agents being read right now; `--kind=agent` lists every agent, `--all` everything you may see.
- `--tag=<tag>` narrows the list to one label.
- `READERS`: reads waiting right now. `QUEUED`: messages waiting for them. `LAST USED`: when the name's credential last made a call.
- 🔍 `READERS 0` with a growing `QUEUED` is an agent that has stopped.
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

- **Set it:** `agent-bus secret db@demo 'PGPASSWORD=s3cret'`
- **From a file:** `agent-bus secret db@demo - < .env`
- **Read it back:** `password=$(agent-bus secret db@demo)`
- What you wrote comes back byte for byte; it must be `KEY=value` lines with `#` comments.
- 🔍 The record's Owner and Maintainers may read it, and an Agent its own; nobody else, and group membership alone grants nothing ([private values](../constitution.md#-private-values)).

### ✏️ Changing a record

- **Add someone unless already there:** `agent-bus manage jobs@demo --add-to-set-allow bob@demo`
- **Take them out:** `agent-bus manage jobs@demo --remove-allow bob@demo`
- **Route a queue to an agent:** `agent-bus manage jobs@demo --deliver-to '#worker@demo'`
- **Hide it:** `agent-bus manage jobs@demo --status inactive`; `--status active` brings it back.
- **Replace the labels:** `agent-bus manage jobs@demo --tags 'work,alerts'`; `--tags=` clears them ([tag rules](../03-records.md#tags)).
- **Set a group's whole membership:** `agent-bus group @crew alice@demo '#worker@demo'`
- Every list (allow, maintainers, deliver-to, tags) takes `--add-`, `--add-to-set-` and `--remove-`; two people editing at once lose nothing.
- An inactive record is in no listing and answers as if absent until reactivated.

## ✉️ Sending and receiving

Three shapes; the only difference is how long you wait.

- **Send, no answer wanted:** `agent-bus send '#echo@demo' "no answer wanted"`
- **Call and wait for the reply:** `agent-bus call '#hi@demo' --wait 10s "ping"`
- **Read your own inbox:** `agent-bus consume --wait 30s`; add `--follow` to keep reading.
- A `call` prints the receipt first, then the answer.

| Word | Means |
|---|---|
| **`ack`** | it arrived and something has it; **not** that the work started |
| **`done`** | the work finished, with nothing to hand back |
| **a reply** | the work finished, and here is the answer |

Answering by hand:

- `agent-bus ack <message-id>`, `agent-bus done <message-id>`, `agent-bus reply <message-id> "here you go"`
- **Unprompted:** `agent-bus reply --to '#hi@demo' --topic t "unprompted"`
- **Answer to another inbox:** `send --reply-to=<name>` keeps the message's `--topic` and `--tag` for matching; the destination must admit the answering agent ([reply routes](../04-messaging.md#request-and-reply)).
- ⚠️ **One reader per inbox.** A silent second reader would look like lost messages, so it is refused; `consume --share` asks for several workers on purpose.

### 📣 Channels, for one-to-many

- **Create one:** `agent-bus register alerts@demo --kind pubsub --descr "shouting"`
- **Publish:** `agent-bus publish --channel alerts@demo "disk is filling up"`
- **Leave it:** `agent-bus unsubscribe alerts@demo`
- ⚠️ **The owner chooses who receives.** You cannot add yourself; `unsubscribe` takes you off, because it is your inbox that fills.
- ⚠️ **A channel is not a topic.** The channel is the name you publish to; `--topic` labels one message ([channels](../03-records-channel.md#channel-not-topic)).

| Kind | Goes to |
|---|---|
| `queue` | **one** consumer, and waits until somebody takes it |
| `pubsub` | **everybody on its [Deliver-To list](../04-messaging.md#deliver-to)**, kept for nobody |

## Checking whether you may send

- `agent-bus may-send '#systemd@node.example'` answers `{"may_send":true}` without sending ([run above](#try-this-first)).
- A refusal prints the reason on stderr and exits nonzero.
- It checks your own identity only, and reserves nothing.
- The [daemon API](../09-daemon-api.md#checking-permission-to-send) owns the rules.

## Asking an agent what it can do

Ask by name; the answer prints as readable text:

```sh
agent-bus help '#systemd@parf.us'
```

```text
Scope: every unit on this host
Inspect units and logs; start, stop and restart units within configured scope
list [type]: List loaded units, including inactive units
  type (service|timer|socket|mount|path|target|slice|scope|automount|swap|device): Optional unit type
timers: List timers with their last and next runs
failed: List failed units
status <unit>: Show one unit’s status
...
More: agent-bus help '#systemd@parf.us' config | secret | json
```

- 🔍 It lists only the methods available **to you**; a caller with more roles sees more.
- **Configuration keys:** `agent-bus help '#systemd@parf.us' config`
- **Secret key names and instructions, never values:** `agent-bus help '#systemd@parf.us' secret`
- **The whole description, pretty-printed:** `agent-bus help '#systemd@parf.us' json`
- 🎨 On a terminal it is coloured; pipes and files get plain text. `NO_COLOR` or `--no-color` turns colour off.
- A refusal or timeout goes to stderr and exits nonzero; `--wait 5s` shortens the 30-second default.

For scripts that want the original message:

- `agent-bus call '#systemd@parf.us' --help-json` returns the message, with the description in its body.
- `call … --help`, `--help-config` and `--help-secret` ask the same way; use one at a time. `send` takes them too.
- `--status` asks for agent status; `--mcp` asks for MCP tools.
- How installed agents supply these answers: [package help](../runner/configuration.md#package-help).

## Calling many agents at once

Ask the same question of every agent with a tag:

```sh
agent-bus ls --tag systemd --table
```

```text
NAME                  KIND     OWNER  READERS  QUEUED  LAST USED  DESCRIPTION
#systemd@parf.us      👾 Agent  parf   1        0       just now   Inspect units and logs; ...
#systemd/web@parf.us  👾 Agent  parf   1        0       just now   Inspect units and logs; ...
```

```sh
agent-bus call --tagged systemd --wait 10s failed
```

```text
## #systemd/web@parf.us
    no failed units

## #systemd@parf.us
    no failed units
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

| Flag | |
|---|---|
| default (`--algo=args`) | the message arrives as an **argument** |
| `--algo=json` | the whole message arrives as **JSON on stdin**: use it to see the sender or topic |
| `-N` (e.g. `-3`) | how many at a time; one by default |
| `--descr=text` | description shown in listings |
| `--allow=names` | who may call it: comma-separated names, `@owner` or `*` |
| `--personal` | a [personal agent](../03-records.md#personal-and-shared) |
| `--tags=a,b` | record labels; none are added for you |
| `--roles=a,b` | roles the script understands; informational, grants nothing |
| `--no-asks` | answer `unsupported: <ask>` without starting the script when metadata cannot answer |
| `--package-dir=/path` | absolute package folder; answers the help asks from its metadata without running the script |
| `--sandbox=on` | confine it; off unless asked |
| `--network` | let a confined script reach the network |
| `--stay-registered` | keep the record and credential at a graceful stop |

- ⚠️ **Use an absolute path.** The script runs in its own work folder, so a relative path is refused; a bare command on `PATH`, such as `echo`, is fine ([script limits](../06-dedicated-runner.md#script-agents)).
- 💡 **It runs in the foreground.** Ctrl-c stops it and, with nothing queued, unregisters the name.
- 🔁 A bus restart or dropped link does not end it: it waits and reconnects ([watching it](dedicated-runner.md#-watching-and-stopping-it)).
- An external supervisor that restarts it uses `--stay-registered` ([lifecycle](../06-dedicated-runner.md#keeping-a-supervised-registration)).
- To keep a script served without a terminal open, use the [runner](../runner/README.md).
- More: [running an agent](dedicated-runner.md).

## 🪪 Joining a bus you are new to

- If the bus trusts a directory that publishes your public key, such as GitHub, introduce yourself: `agent-bus enrol me@github --key ~/.ssh/id_ed25519`
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
- Scripts receive it as compact `AGENT_BUS_CONFIG` at their next start ([`jq` example](../03-records-agent.md#configuring-a-template)).

## 🆘 When something looks wrong

| You see | It usually means |
|---|---|
| `set AGENT_BUS_TOKEN` | you are on the shared socket, not your own; see [above](#-how-it-finds-the-bus-and-how-it-knows-you) |
| `no inbox for …: register it first` | you asked to `consume` from a name nobody registered; a User's own inbox always exists |
| `no answer within 30s` | the message *was* accepted; do not send it again. Nobody answered in time |
| a name that "does not exist" | sending to an unregistered name is refused on purpose, so you learn now |
| `ls --kind=agent` shows `READERS 0` and `QUEUED` climbing | whatever serves that name is not running |

- `agent-bus status` shows the counters worth watching: `dropped`, `expired` and `refused`.

## 👤 For the bus operator

`agent-bus-admin` runs as the account the bus runs as:

- **Let somebody in:** `agent-bus-admin user add parf@myhost ~/.ssh/id_ed25519.pub`
- **As a bus operator:** `agent-bus-admin user add parf@myhost - --admin < key.pub`
- **Fill a name from a local account:** `agent-bus-admin user import-local parf@myhost parf`
- **A keyless user on that account's socket:** `agent-bus-admin user add-local worker@myhost worker`
- **List and remove:** `agent-bus-admin user list`, `agent-bus-admin user remove parf@myhost`
- **Rotate a credential:** `agent-bus-admin token parf@myhost --rotate`
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
- The older `agent-template`, `register --addr` and `channel create` still work but are hidden from help.

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

<details>
<summary>Record backups</summary>

- Export and import ([how-to](backup.md), [details](../07-setup.md#encrypted-record-backups)) need the bus Owner's own login socket.
- They keep permanent settings and credentials in an encrypted archive.
- They never include queued messages or held locks.

</details>
