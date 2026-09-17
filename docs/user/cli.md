# 🧰 The command-line tools

📌 **TL;DR:** Use `agent-bus` to register, send, read and run.

Everything you do with agent-bus from a terminal. Start here.

There are five programs. You will use the first one almost always:

| Program | It is for | You are |
|---|---|---|
| **`agent-bus`** | 💬 everyday use — register things, send messages, run a service | anyone |
| **`agent-bus-token`** | 🎟️ getting your credential | anyone |
| **`agent-bus-admin`** | 👤 adding people, handing out credentials | the operator |
| **`agent-bus-setup`** | 📦 installing the whole thing once | root, once |
| **`agent-busd`** | ⚙️ the daemon itself — see [the daemon](daemon.md) | it runs itself |

## 🚀 Your first five minutes

Assuming somebody has installed it already ([setup](../09-setup.md#install)):

```sh
agent-bus status                      # is it alive, and who does it think I am?
agent-bus register me@myhost          # give myself an inbox
agent-bus ls -h                       # what is on this bus?
agent-bus send me@myhost "hello"      # talk to myself
agent-bus consume --wait 5s           # and read it back
```

`status` is the one to try first. It answers with your name, so it tells you
two things at once — the bus is up, **and** it knows who you are:

```json
{"up":"6s","services":2,"queued":1,"you":"me@demo","administrator":true}
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
actually replace it, say `--rotate`; the old one keeps working for a while so
messages already in flight are not stranded.

## 📇 Registering things

Nothing exists on the bus until somebody says it does, and **registering is
just stating a description**. The thing described does not have to know the
bus exists.

```sh
agent-bus register echo@demo --descr "says it back"
agent-bus ls -h
agent-bus unregister echo@demo
```

```
NAME       KIND     OWNER    READER  QUEUED  DESCRIPTION
me@demo    generic  me@demo  no      0       just me
hi@demo    generic  me@demo  yes     0       greets
echo@demo  generic  me@demo  no      1       says it back
```

`READER` says whether anything is actually listening, and `QUEUED` how much is
waiting for it. A row with `no` and a growing number is the picture of a
service that has stopped. 🔍

Useful extras when you register:

| Flag | |
|---|---|
| `--descr` | what it is, in a few words. This is what everyone else sees |
| `--allow a@b,c@d` or `--allow '*'` | who may use it. Leave it out and the bus decides |
| `--ttl 1h` · `--bound 1000` | how long its queue keeps things, and how much of it |
| `--overflow ring\|strict` | when full: drop the oldest, or refuse new ones |
| `--addr` · `--protocol` | where it really lives, if it is not an ordinary bus service |

ℹ️ `unregister` removes the **entry**, not the process. If something is still
serving that name, it will register itself again.

## ✉️ Sending and receiving

Three shapes, and the difference is only how long you wait:

```sh
agent-bus send  echo@demo "no answer wanted"
agent-bus call  hi@demo --wait 10s "ping"        # wait for the reply
agent-bus consume --wait 30s                     # read my own inbox
agent-bus consume --follow                       # ...and keep reading
```

A `call` shows you the receipt first, then the answer:

```
ack from hi@demo
{"message_id":"573c…","from":"hi@demo","body":"you said: ping", …}
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
agent-bus reply --to someone@host --topic t "unprompted"
```

⚠️ **One reader per inbox.** A second `consume` on the same name is **refused**
rather than quietly sharing — because a silent second reader looks exactly
like messages going missing. If you *do* want several workers behind one name,
say so: `consume --share`.

### 📣 Topics, for one-to-many

```sh
agent-bus topic create alerts@demo --kind pubsub --descr "shouting"
agent-bus subscribe alerts@demo
agent-bus publish --topic alerts@demo "disk is filling up"
agent-bus unsubscribe alerts@demo
```

| Kind | Goes to |
|---|---|
| `queue` | **one** consumer, and waits until somebody takes it |
| `pubsub` | **every** current subscriber, and is kept for nobody |

## ⚙️ Running a script as a service

The shortest way to put something on the bus: one command line, and your
script is a service.

```sh
agent-bus start hi@demo --algo=args /full/path/to/hi.sh --descr "greets"
```

Now `agent-bus call hi@demo "ping"` runs the script, and whatever it prints on
stdout comes back as the answer.

| Flag | |
|---|---|
| `--algo=args` | the message arrives as an **argument** |
| `--algo=json` | the whole envelope arrives as **JSON on stdin** — use this when you care about who sent it, or the topic |
| `-N` (e.g. `-3`) | how many at a time. One by default |
| `--sandbox on` | confine it. Off unless asked |
| `--network` | let it reach the network. Not unless asked |

```sh
agent-bus logs hi@demo --lines 50 --follow
agent-bus stop hi@demo
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
| `no inbox for you@host: register it first` | you tried to `consume` before registering **your own** name |
| `no answer within 30s` | the message *was* accepted — do not send it again. Nobody answered in time |
| a name that "does not exist" | sending to an unregistered name is refused on purpose, so you learn now rather than later |
| `ls` shows `READER no` and `QUEUED` climbing | whatever serves that name is not running |

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
[the daemon](daemon.md). Putting a script on the bus is [running a
service](runner.md); talking to a live AI session is [Claude Code, Codex and
opencode](agents.md).
