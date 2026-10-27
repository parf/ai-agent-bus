# 🧰 The command-line tools

📌 **TL;DR:** Use `agent-bus` to register, send, read and run — everything you
do with the bus from a terminal. There are six programs, and the first covers
almost all everyday use; the others get you a credential, add people, and
install the whole thing once.

You will use the first one almost always:

| Program | It is for | You are |
|---|---|---|
| **`agent-bus`** | 💬 everyday use — register things, send messages, run an agent | anyone |
| **`agent-bus-token`** | 🎟️ getting your credential | anyone |
| **`agent-bus-admin`** | 👤 adding people, handing out credentials | the operator |
| **`agent-bus-setup`** | 📦 installing the whole thing once | root, once |
| **`agent-bus-runner`** | installed scripts, pools and reloads — see [runner guide](../runner/README.md) | the runner account |
| **`agent-busd`** | ⚙️ the daemon itself — see [the daemon](daemon.md) | it runs itself |

## Option spelling

Write long options as `--name` or `--name=value`. A single dash is for one-letter options such as `-h`. Older single-dash long spellings remain accepted for compatibility.

## 🚀 Your first five minutes

Assuming somebody has installed it already ([setup](../07-setup.md#install)):

```sh
agent-bus help                        # every verb; opens with agent-bus <version>
agent-bus status                      # is it alive, and who does it think I am?
agent-bus ls -h                       # which agents can take a message now? --all: everything
agent-bus send me@myhost "hello"      # talk to myself — a User's inbox already exists
agent-bus consume --wait 5s           # and read it back
```

`me@myhost` stands for the name `status` reports as `you`.

`status` is the one to try first. It answers with your name, so it tells you
two things at once — the bus is up, **and** it knows who you are:

```json
{"up":"6s","records":2,"queued":0,"waiting":0,"dropped":0,"expired":0,…,"you":"me@demo","administrator":true}
```

✅ If you see your name, you are done setting up. Nothing else to configure.

## 🔌 How it finds the bus, and how it knows you

You usually do not have to think about either.

| | |
|---|---|
| **on your own machine** | `agent-bus` finds your socket by itself. The socket *is* your credential — the operating system already knows which account opened it, so **no token is needed** |
| **from somewhere else** | set `AGENT_BUS_ADDR` to `http://host:port` and `AGENT_BUS_TOKEN` to your credential, or forward your socket over SSH ([remote access](../02-access-remote.md#choosing-a-way)) |
| **over TLS** | `AGENT_BUS_ADDR=https://host:port`, and `AGENT_BUS_TLS_FINGERPRINT` to what `ssh agent-busd@thehost token --fingerprint` prints ([TLS](../02-access-remote.md#over-https)) |
| **pointing somewhere specific** | `--addr <socket-path>` or `--addr http://host:port` beats both |

⚠️ `set AGENT_BUS_TOKEN` as an answer means you reached the **shared** socket
rather than your own. Either you have no per-account socket on this bus, or
`AGENT_BUS_ADDR` is pointing at the wrong one. Ask the operator to add you
(they run `agent-bus-admin user add`), or set a token.

Get a credential with its own little program:

```sh
export AGENT_BUS_TOKEN=$(ssh agent-busd@thehost token)   # over ssh
agent-bus-token me@myhost                                # on the box
agent-bus-token me@myhost --key ~/.ssh/id_ed25519        # with a key, no ssh
```

💡 Asking twice gives you the **same** token — it is a read, not a reset. To
actually replace it, say `--rotate`; the old one keeps working until the next
rotation, so callers still holding it are not stranded.

## 📇 Registering things

Nothing exists on the bus until somebody says it does, and **registering is
just stating a description**. The thing described does not have to know the
bus exists.

⚠️ **An agent's name begins with `#`**, which a shell reads as a comment:
quote it (`'#echo@demo'`) or write `--agent echo@demo` wherever a name goes.
A name without `#` and without `--kind` registers a 📡 service, which needs
`--addr` and `--protocol`.

```sh
agent-bus register '#echo@demo' --descr "says it back"
agent-bus ls -h --all
agent-bus unregister '#echo@demo'
```

Plain `ls` lists only the agents being read right now; `--kind agent` lists
every agent, `--kind queue` every queue, and `--all` everything you may see.

```
NAME         KIND        OWNER    READERS  QUEUED  LAST USED  DESCRIPTION
#echo@demo   👾 Agent    me@demo  0        1       -          says it back
#hi@demo     👾 Agent    me@demo  1        0       just now   greets
alerts@demo  📣 PubSub   me@demo  0        0       -          shouting
db@demo      📡 Service  me@demo  -        -       -          the database
me@demo      👤 User     me@demo  0        0       -
```

`READERS` counts reads waiting right now, `QUEUED` how much is waiting for
them, and `LAST USED` when the name's credential last made a call. A row with
`READERS 0` and a growing `QUEUED` is the picture of an agent that has
stopped. 🔍 A 📡 and a 👥 group show `-` for both: a service is
[something outside](../03-records-service.md#what-a-service-is) with no queue here,
and a group is a list of members, so there is nothing to count.

Each command accepts only its own flags. An unknown or misplaced `--flag`
is refused by name with a non-zero exit before any request reaches the daemon.
Boolean switches do not consume the following name or text. A valued flag
without its argument is refused too; use `--descr=` to supply an explicit
empty value when clearing a description. A bare `--` ends options: everything
after it is literal, including `--agent` and words starting with `--`.

```sh
agent-bus send '#worker@demo' -- "--verbose please"
```

Useful extras when you register:

| Flag | |
|---|---|
| `--descr` | what it is, in a few words. This is what everyone else sees |
| `--allow a@b,c@d` or `--allow '*'` | who may use it. Leave it out and the bus decides |
| `--ttl 1h` · `--bound 1000` | how long an agent's or queue's inbox keeps things, and how much of it. A 📣 pubsub takes neither: each copy lives by its recipient's TTL |
| `--overflow ring\|strict` | when full: drop the oldest, or refuse new ones |
| `--addr` · `--protocol` | where it really lives — required when the record is a 📡 [service](../03-records-service.md#how-to-call-it), which is something outside |

ℹ️ `unregister` removes the **entry**, not the process. If something is still
serving that name, it will register itself again.

### 🔐 The credential for something outside

A 📡 [service](../03-records-service.md#what-a-service-is) is called by you rather
than by the bus, and reaching it usually takes a password or a token. Keep it
on the record, and whoever that record already admits can read it:

```sh
agent-bus secret db@demo 'PGPASSWORD=s3cret'   # set it
agent-bus secret db@demo - < .env              # or from a file
password=$(agent-bus secret db@demo)           # read it back
```

What you wrote is what comes back, byte for byte. It must be an env file —
`KEY=value` lines, `#` comments — and the bus checks that much and never looks
further inside. 🔍 The record's Owner and Maintainers may read it, and an Agent may read its own.
Other callers receive neither its bytes nor its digest. Group membership alone
does not grant access. See [private values](../constitution.md#-private-values).

### ✏️ Changing a record

```sh
agent-bus manage jobs@demo --add-to-set-allow bob@demo        # add, unless already there
agent-bus manage jobs@demo --remove-allow bob@demo            # take it out
agent-bus manage jobs@demo --deliver-to '#worker@demo'        # route the queue to an agent that allows it
agent-bus manage jobs@demo --status inactive                  # hide it; --status active brings it back
agent-bus manage jobs@demo --tags 'work,alerts'                # replace the labels
agent-bus manage jobs@demo --add-to-set-tags urgent            # add one unless present
agent-bus manage jobs@demo --remove-tags urgent                # remove that label
agent-bus group @crew alice@demo '#worker@demo'               # a group's whole membership
```

Every list — allow, maintainers, deliver-to and tags — takes `--add-`, `--add-to-set-`
and `--remove-`, applied to the record as the bus finds it, so two people
editing at once lose nothing. `--tags` replaces the whole tag list; `--tags=`
clears it ([tag rules](../03-records.md#tags)). An inactive record is in no listing and answers
as if it did not exist until it is reactivated.

## ✉️ Sending and receiving

Three shapes, and the difference is only how long you wait:

```sh
agent-bus send  '#echo@demo' "no answer wanted"
agent-bus call  '#hi@demo' --wait 10s "ping"     # wait for the reply
agent-bus consume --wait 30s                     # read my own inbox
agent-bus consume --follow                       # ...and keep reading
```

A `call` shows you the receipt first, then the answer:

```
ack from #hi@demo
{"message_id":"5b70…","from":"#hi@demo","to":"me@demo","topic":"call","body":"you said: ping", …}
```

| Word | Means |
|---|---|
| **`ack`** | it arrived and something has it. **Not** that the work started |
| **`done`** | the work finished, with nothing to hand back |
| **a reply** | the work finished, and here is the answer |

Answering by hand:

```sh
agent-bus ack  <message-id>
agent-bus done <message-id>
agent-bus reply <message-id> "here you go"
agent-bus reply --to '#hi@demo' --topic t "unprompted"
```

⚠️ **One reader per inbox.** A second `consume` on the same name is **refused**
rather than quietly sharing — because a silent second reader looks exactly
like messages going missing. If you *do* want several workers behind one name,
say so: `consume --share`.

`send --reply-to <name>` directs the answer to another inbox, keeping the
message’s `--topic` and `--tag` for matching. The destination still has to
admit the answering agent ([reply routes](../04-messaging.md#request-and-reply)):

```sh
agent-bus send '#worker@demo' --reply-to '#reviewer@demo' --topic report --tag run1 "check the report"
```

### 📣 Channels, for one-to-many

```sh
agent-bus channel create alerts@demo --kind pubsub --descr "shouting"
agent-bus publish --channel alerts@demo "disk is filling up"
agent-bus unsubscribe alerts@demo
```

⚠️ **Who receives is the channel owner's list.** You cannot add yourself to a
📣 — its owner sets who it delivers to, on the channel's page. `unsubscribe`
takes *you* off, because it is your inbox that fills.

⚠️ **A channel is not a topic.** The channel is the *name* you publish to; a
`--topic` on `send` is a **label on one message**, used to match a reply. Two
words, two things ([channels](../03-records-channel.md#channel-not-topic)).

| Kind | Goes to |
|---|---|
| `queue` | **one** consumer, and waits until somebody takes it |
| `pubsub` | **everybody on its [Deliver-To list](../04-messaging.md#deliver-to)**, and is kept for nobody |

## Checking whether you may send

Check your permission without sending a message:

```sh
agent-bus may-send '#systemd@node.example'
# {"may_send":true}
```

A refusal prints the bus's reason on stderr and exits nonzero. The command
checks your own identity; it cannot check for somebody else. It does not
reserve a place in the queue. The [daemon API](../09-daemon-api.md#checking-permission-to-send)
owns the checks and their error rules.

## Asking an agent what it can do

Start with the agent's name. The CLI prints its answer as readable text, without
the message wrapper:

```sh
agent-bus help '#systemd@parf.us'
agent-bus help '#systemd@parf.us' config
agent-bus help '#systemd@parf.us' secret
agent-bus help '#systemd@parf.us' json
```

The first form lists methods and arguments available to you. `config` explains
Configuration keys; `secret` lists Secret key names and instructions, never saved
values. `json` pretty-prints the whole description. For example, a caller allowed
to restart services sees this excerpt:

```text
restart <unit>: Restart a unit within this instance’s scope; role: operate|admin; destructive
  unit (string): Exact fully qualified Linux unit name, for example nginx.service
```

On a terminal, headings and method names are bold, roles are yellow, destructive
marks are red, and required Secret keys are marked. JSON has syntax colours.
Pipes and files receive plain output, with no added ANSI codes. Set `NO_COLOR`
or pass `--no-color` to turn colours off on a terminal too:

```sh
agent-bus help '#systemd@parf.us' --no-color
agent-bus help '#systemd@parf.us' json > systemd-help.json
```

A refusal or timeout goes to stderr and exits nonzero. `--wait 5s` can shorten
the default 30-second wait.

For scripts that need the message wrapper, keep using
`agent-bus call '#systemd@parf.us' --help-json`. It returns the original JSON
message, whose `body` contains the description. The existing `call … --help`,
`--help-config` and `--help-secret` forms are unchanged. These flags send one ask
with an empty body; use one at a time. The same flags work with `send`.
`--mcp` remains the ask for MCP tools. See [package help](../runner/configuration.md#package-help)
for how installed agents supply these answers.

## Calling many agents at once

Use a record tag to ask the same question of several agents:

```sh
agent-bus ls --tag systemd
agent-bus call --tagged systemd --wait 10s failed
agent-bus call --tagged systemd --wait 10s "logs agent-bus-web.service 50 since 1h"
```

`call --tagged <tag> [--wait D] <body>` selects **only caller-visible agents**
from the daemon's tag-filtered listing, including agents with no current reader.
Other record kinds, untagged agents and hidden agents are never called. Tags
label records; they do not grant access or prove that an agent is healthy.
`ls --tag <tag>` lists every visible matching record, readers or not, and keeps
its usual JSON/table output.

The tagged form takes **exactly one body argument**: quote a multi-word body.
It refuses an explicit target and the other call flags beside `--wait`.
The selector is `--tagged` because the existing single-target `--tag` is an
exchange-correlation value; `call <target> --topic t --tag g <body>` keeps its
meaning.

Calls run concurrently under **one shared
`--wait` deadline** (30 seconds by default), rather than a new wait per target.
Each target gets a fresh topic and correlation tag. The accepted request is
never retried: a missing answer does not prove that an operation did nothing.
The usual [inbox reader limit](../04-messaging.md#one-reader-per-inbox) applies;
use a caller identity whose inbox another reader is not draining.

After the wait, successful answer bodies appear under name-sorted headings.
Failures follow, also sorted by name within each list:

```text
## #systemd/a@host
<answer body>

## #systemd/b@host
<answer body>

No answer:
- #systemd/c@host: no answer within 10s (the message may have been accepted; do not resend it)
Refused:
- #systemd/d@host: <daemon refusal of the send>
```

Exit **0** means every selected agent answered; **nonzero** means a no-answer,
a refusal, a lookup failure, invalid arguments, or no matching agents. A `done`
receipt goes in the no-answer list with its explicit completion reason, without
waiting out the deadline. Transport or consume failures also go under no-answer:
the outcome may be unknown, so do not blindly resend. Answer bodies remain opaque: an application's own
JSON error is still an answer, as with a single-target call.

## ⚙️ Running a script as an agent

The shortest way to put something on the bus: one command line, and your
script is an agent — the kind with a queue something reads.

```sh
agent-bus run '#hi@demo' /full/path/to/hi.sh --descr "greets"
```

Now `agent-bus call '#hi@demo' "ping"` runs the script, and whatever it prints on
stdout comes back as the answer.

| Flag | |
|---|---|
| Default (`args`) | the message arrives as an **argument** |
| `--algo=json` | the whole envelope arrives as **JSON on stdin** — use this when you care about who sent it, or the message's topic |
| `-N` (e.g. `-3`) | how many at a time. One by default |
| `--descr text` | description shown in listings |
| `--allow names` | who may call it; comma-separated names, `@owner` or `*` |
| `--personal` | register it as a [personal agent](../03-records.md#personal-and-shared) |
| `--tags a,b` | explicit record labels; no automatic tag is added |
| `--roles a,b` | application roles the script understands; informational, grants none by itself |
| `--no-asks` | answer `unsupported: <ask>` without starting the script when metadata cannot answer |
| `--package-dir /path` | absolute package directory; strictly load metadata and answer the four help asks without running the script |
| `--sandbox on` | sandbox it. Off unless asked |
| `--network` | let it reach the network. Not unless asked |

```sh
agent-bus logs '#hi@demo' --lines 50 --follow
agent-bus stop '#hi@demo'
```

⚠️ **Use an absolute path for the script.** The child runs in a work directory
of its own, so `run` refuses any relative path. A bare command on `PATH`,
such as `echo`, is allowed. The executable path must be unquoted and contain
no whitespace; its arguments may use shell quoting
([script argument limits](../06-dedicated-runner.md#script-agents)).

💡 It runs in the **foreground**. Ctrl-c stops it and, with nothing queued,
unregisters the name. Use `--stay-registered` when an external supervisor
restarts it with the agent's own token; graceful stop then retains its record
and credential ([lifecycle](../06-dedicated-runner.md#keeping-a-supervised-registration)). A daemon restart or a dropped link does not end it: it
waits and reconnects by itself ([runner](dedicated-runner.md#-watching-and-stopping-it)).
A script that restarts after a crash is the
[runner](../../Plans/R1.0-Release/runner.md#runner), R1.

## 🪪 Joining a bus you are new to

If the bus knows a directory that publishes your public key — GitHub, for
instance — you can introduce yourself:

```sh
agent-bus enrol me@github --key ~/.ssh/id_ed25519
```

The bus sends a challenge, your key signs it, and you get an ordinary record.
Fetching a public key is not proof of anything; **signing the challenge is**.

## 🆘 When something looks wrong

| You see | It usually means |
|---|---|
| `set AGENT_BUS_TOKEN` | you are on the shared socket, not your own — see [above](#-how-it-finds-the-bus-and-how-it-knows-you) |
| `no inbox for …: register it first` | you asked to `consume` from a name nobody registered. A User's own inbox always exists |
| `no answer within 30s` | the message *was* accepted — do not send it again. Nobody answered in time |
| a name that "does not exist" | sending to an unregistered name is refused on purpose, so you learn now rather than later |
| `ls --kind agent` shows `READERS 0` and `QUEUED` climbing | whatever serves that name is not running; plain `ls` lists only agents being read |

Ask the daemon how it is doing with `agent-bus status` — `dropped`, `expired`
and `refused` there are the counters worth watching.

## 👤 For the operator

`agent-bus-admin` is the same shape, for the account the daemon runs as:

```sh
agent-bus-admin user add parf@myhost ~/.ssh/id_ed25519.pub   # let somebody in
agent-bus-admin user add parf@myhost - --admin < key.pub     # ...as an operator
agent-bus-admin user import-local parf@myhost parf           # fill a blank name from passwd
agent-bus-admin user add-local worker@myhost worker  # a keyless user, its socket that account's
agent-bus-admin user list
agent-bus-admin user remove parf@myhost
agent-bus-admin token parf@myhost --rotate
```

A key added this way reaches **one command and no shell** — `agent-bus-token`
for an ordinary person, this program for an operator. Installing the whole
thing is [setup](../07-setup.md#install), and what the daemon itself wants is
[the daemon](daemon.md). Putting a script on the bus is [running an
agent](dedicated-runner.md); talking to a live AI session is [Claude Code, Codex and
opencode](ai-cli.md).

## Record storage and locks

For a record you may manage, these commands read/write its stored values or
coordinate work. See [KV](../03-records-kv.md#key-value-store) and
[locks](../01-identity-and-authority.md#shared-locks) for all options and limits.

| Command | Result |
|---|---|
| `agent-bus kv get jobs@demo cursor --int` | Read an integer |
| `agent-bus kv set jobs@demo cursor 42 --int` | Store an integer |
| `agent-bus kv inc jobs@demo cursor` | Add one atomically |
| `agent-bus kv delete jobs@demo cursor --int` | Remove it |
| `agent-bus kv json jobs@demo batch '[{"op":"push","key":"todo","value":"job-1"}]'` | Edit a JSON value atomically |
| `agent-bus lock jobs@demo deploy --ttl=30s --wait=10s` | Wait for a named lock |
| `agent-bus try-lock jobs@demo deploy --ttl=30s` | Try immediately |
| `agent-bus extend jobs@demo deploy --ttl=30s` | Renew your hold |
| `agent-bus holders jobs@demo` | List current holds |
| `agent-bus release jobs@demo deploy` | Release your hold |

## Agent configuration

Keep secrets in Secret and config in Configuration; see [configuration](../03-records-agent.md#configuring-a-template).

`agent-bus agent-template <name>` reads its JSON settings; add JSON or `-`
(stdin) to replace them as Owner or Maintainer. An Agent may read its own.
Configuration is not a Secret: authorized web pages display it, while other
callers get neither bytes nor digest. Scripts receive compact `AGENT_BUS_CONFIG`
at their next start, unset when unconfigured. See the
[`jq` example](../03-records-agent.md#configuring-a-template).
