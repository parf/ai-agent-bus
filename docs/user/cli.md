# 🧰 The command-line tools

📌 **TL;DR:** Use `agent-bus` to register, send, read and run — everything you
do with the bus from a terminal. There are five programs, and the first covers
almost all everyday use; the others get you a credential, add people, and
install the whole thing once.

You will use the first one almost always:

| Program | It is for | You are |
|---|---|---|
| **`agent-bus`** | 💬 everyday use — register things, send messages, run an agent | anyone |
| **`agent-bus-token`** | 🎟️ getting your credential | anyone |
| **`agent-bus-admin`** | 👤 adding people, handing out credentials | the operator |
| **`agent-bus-setup`** | 📦 installing the whole thing once | root, once |
| **`agent-busd`** | ⚙️ the daemon itself — see [the daemon](daemon.md) | it runs itself |

## 🚀 Your first five minutes

Assuming somebody has installed it already ([setup](../09-setup.md#install)):

```sh
agent-bus status                      # is it alive, and who does it think I am?
agent-bus ls -h                       # what is on this bus?
agent-bus send me@myhost "hello"      # talk to myself — a User's inbox already exists
agent-bus consume --wait 5s           # and read it back
```

`me@myhost` stands for the name `status` reports as `you`.

`status` is the one to try first. It answers with your name, so it tells you
two things at once — the bus is up, **and** it knows who you are:

```json
{"up":"6s","services":2,"queued":0,"waiting":0,"dropped":0,"expired":0,…,"you":"me@demo","administrator":true}
```

✅ If you see your name, you are done setting up. Nothing else to configure.

## 🔌 How it finds the bus, and how it knows you

You usually do not have to think about either.

| | |
|---|---|
| **on your own machine** | `agent-bus` finds your socket by itself. The socket *is* your credential — the operating system already knows which account opened it, so **no token is needed** |
| **from somewhere else** | set `AGENT_BUS_ADDR` to `http://host:port` and `AGENT_BUS_TOKEN` to your credential |
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
agent-bus ls -h
agent-bus unregister '#echo@demo'
```

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
[something outside](../06-services.md#what-a-service-is) with no queue here,
and a group is a list of members, so there is nothing to count.

Useful extras when you register:

| Flag | |
|---|---|
| `--descr` | what it is, in a few words. This is what everyone else sees |
| `--allow a@b,c@d` or `--allow '*'` | who may use it. Leave it out and the bus decides |
| `--ttl 1h` · `--bound 1000` | how long an agent's or queue's inbox keeps things, and how much of it. A 📣 pubsub takes neither: each copy lives by its recipient's TTL |
| `--overflow ring\|strict` | when full: drop the oldest, or refuse new ones |
| `--addr` · `--protocol` | where it really lives — required when the record is a 📡 [service](../06-services.md#how-to-call-it), which is something outside |

ℹ️ `unregister` removes the **entry**, not the process. If something is still
serving that name, it will register itself again.

### 🔐 The credential for something outside

A 📡 [service](../06-services.md#what-a-service-is) is called by you rather
than by the bus, and reaching it usually takes a password or a token. Keep it
on the record, and whoever that record already admits can read it:

```sh
agent-bus secret db@demo 'PGPASSWORD=s3cret'   # set it
agent-bus secret db@demo - < .env              # or from a file
password=$(agent-bus secret db@demo)           # read it back
```

What you wrote is what comes back, byte for byte. It must be an env file —
`KEY=value` lines, `#` comments — and the bus checks that much and never looks
further inside. 🔍 Nobody else is shown it: every listing, every record answer
and the web page carry a **digest** of it instead, so you can see that it
changed without being handed it. A 📡 service, an 👾 agent (read by that agent
alone) and a 👥 group (read by its members) have one.

### ✏️ Changing a record

```sh
agent-bus manage jobs@demo --add-to-set-allow bob@demo        # add, unless already there
agent-bus manage jobs@demo --remove-allow bob@demo            # take it out
agent-bus manage jobs@demo --deliver-to '#worker@demo'        # route the queue to an agent that allows it
agent-bus manage jobs@demo --status inactive                  # hide it; --status active brings it back
agent-bus group @crew alice@demo '#worker@demo'               # a group's whole membership
```

Every list — allow, maintainers, deliver-to — takes `--add-`, `--add-to-set-`
and `--remove-`, applied to the record as the bus finds it, so two people
editing at once lose nothing. An inactive record is in no listing and answers
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
words, two things ([channels](../07-channels.md#channel-not-topic)).

| Kind | Goes to |
|---|---|
| `queue` | **one** consumer, and waits until somebody takes it |
| `pubsub` | **everybody on its [Deliver-To list](../04-messaging.md#subscribers)**, and is kept for nobody |

## ⚙️ Running a script as an agent

The shortest way to put something on the bus: one command line, and your
script is an agent — the kind with a queue something reads.

```sh
agent-bus start '#hi@demo' --algo=args /full/path/to/hi.sh --descr "greets"
```

Now `agent-bus call '#hi@demo' "ping"` runs the script, and whatever it prints on
stdout comes back as the answer.

| Flag | |
|---|---|
| `--algo=args` | the message arrives as an **argument** |
| `--algo=json` | the whole envelope arrives as **JSON on stdin** — use this when you care about who sent it, or the message's topic |
| `-N` (e.g. `-3`) | how many at a time. One by default |
| `--sandbox on` | confine it. Off unless asked |
| `--network` | let it reach the network. Not unless asked |

```sh
agent-bus logs '#hi@demo' --lines 50 --follow
agent-bus stop '#hi@demo'
```

⚠️ **Use an absolute path for the script.** The child runs in a work directory
of its own, so `./hi.sh` is not where you think it is — the failure looks like
`exited badly … exit status 127`, which `agent-bus logs` will show you.

💡 It runs in the **foreground** and stops when you press ctrl-c. Something
that should come back by itself after a reboot wants
[the runner](runner.md).

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
| `ls` shows `READERS 0` and `QUEUED` climbing | whatever serves that name is not running |

Ask the daemon how it is doing with `agent-bus status` — `dropped`, `expired`
and `refused` there are the counters worth watching.

## 👤 For the operator

`agent-bus-admin` is the same shape, for the account the daemon runs as:

```sh
agent-bus-admin user add parf@myhost ~/.ssh/id_ed25519.pub   # let somebody in
agent-bus-admin user add parf@myhost - --admin < key.pub     # ...as an operator
agent-bus-admin user import-local parf@myhost parf           # fill a blank name from passwd
agent-bus-admin user list
agent-bus-admin user remove parf@myhost
agent-bus-admin token parf@myhost --rotate
```

A key added this way reaches **one command and no shell** — `agent-bus-token`
for an ordinary person, this program for an operator. Installing the whole
thing is [setup](../09-setup.md#install), and what the daemon itself wants is
[the daemon](daemon.md). Putting a script on the bus is [running an
agent](runner.md); talking to a live AI session is [Claude Code, Codex and
opencode](agents.md).
