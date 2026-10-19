// The agent's self-description, the answer to `--help`
// (docs/constitution.md#agent-self-description): what it is for, every
// method with its arguments, answer and an example, written for an AI
// reader — and as the caller may use it: subscriptions are changed by whoever
// manages their destination, and only this agent's own Owner and Maintainers
// are told they may change any (Plans/R1.2/slack/README.md#subscriptions-authority-errors).

const methods = (who: string) => `## Methods

Each call is one text line, or the same call as JSON. A JSON call is answered
in JSON, Slack's own shape: {"ok": true, …} or {"ok": false, "error": "…"}.
A text call is answered as text; a failure is "error: <why>". <#chan|CID> is a
channel's #name or its id (C…); <ts> is a message's Slack timestamp, as read
answers it.

- post <#chan|CID|email> <text…>
  Posts a message; an email address posts a DM to that person. Answers the
  message's permalink. JSON: {"method":"post","channel","text","thread_ts"?,"username"?,"icon"?};
  thread_ts replies in a thread; username and icon (":emoji:" or an image URL)
  change how this one post is signed.
  Example: agent-bus call '${who}' 'post #dev deploy finished'
- read <#chan|CID> [N]
  The last N messages (default 20, at most 200), oldest first, one per line:
  "<ts> @<name> <text>". JSON: {"method":"read","channel","limit"?}; each
  message keeps the raw user id and text and adds user_name.
  Example: agent-bus call '${who}' 'read #dev 5'
- thread <#chan|CID> <ts> [N]
  One thread's messages, oldest first. JSON: {"method":"thread","channel","ts","limit"?}
- channels [filter]
  The channels: id, #name, members, whether the bot is a member, topic;
  filter is a substring of the name. JSON: {"method":"channels","filter"?}
- channel <#chan|CID>
  One channel: id, name, topic, purpose, member count, created.
- users [filter]
  The workspace's people, without bots and deleted accounts: id, name, real
  name, email; filter matches any of them. JSON: {"method":"users","filter"?}
- user <id|email|@name>
  One person: id, names, email, time zone, title, whether a bot.
- react <#chan|CID> <ts> <emoji>
  Adds a reaction, e.g. white_check_mark, to mark a message handled.
- edit <#chan|CID> <ts> <text…>, delete <#chan|CID> <ts>
  Edit or delete a message; Slack allows it only for the bot's own posts.
- join <#chan|CID>
  Join a public channel, so the bot can read it.
- whoami
  The bot's Slack identity and workspace.
- sub list
  The subscriptions: id, channel, target, from_users, filter, who made it, how many
  messages it forwarded, its last error.`;

const changes = (who: string, any: boolean) => `
Adding, editing and deleting a subscription is for whoever manages its
destination (its Owner or a Maintainer); moving one needs the new destination
too. ${any ? "As this agent's Owner or a Maintainer, you may change any subscription." : "Others' subscriptions are not yours to change."}

- sub add <#chan|CID> <target> [from=user,…] [filter]
  Forward a channel's new messages to a bus agent, queue or pubsub.
  from= names authors by user ID, email, @name, or bot ID; names resolve
  before storage. Unknown authors are refused. An absent list accepts all. filter
  is a regex tested against the text: /pattern/flags (e.g. /deploy/i), or any
  other text as a pattern without flags (e.g. ^agent: ); it runs to the end of
  the line. Answers the subscription's id.
  Example: agent-bus call '${who}' 'sub add #dev #worker@realm /error/i'
- sub edit <id> [channel=…] [target=…] [from=…] [filter=…]
  Changes only the fields named; filter= is the last and runs to the end of
  the line; an empty filter= clears it. from= alone clears the author list;
  JSON uses from_users: [] to clear, or an array of names to replace it.
- sub delete <id>
  Removes it; nothing more is forwarded.`;

const forwarding = (who: string) => `## Forwarded messages

A subscription's target receives each matching message from ${who}, topic
"slack", tag the channel id, body JSON:
{"channel","channel_name","user","user_name"?,"text","ts","thread_ts"?}.
The target's ACL must admit ${who}. Edits, deletions and the agent's own
posts are not forwarded.`;

/** The description, as the caller may use it: `any` when it may change every subscription. */
export function describe(who: string, any: boolean): string {
  return [
    `# ${who}: Slack on the bus`,
    "",
    `One Slack workspace, behind one bus agent: post and read messages, look up
channels and people, and forward a channel's messages to a bus agent, queue or
pubsub, filtered by a regex. Call it with agent-bus call '${who}' '<method …>'
or by sending it the same text and reading the answer.`,
    "",
    methods(who),
    changes(who, any),
    "",
    forwarding(who),
  ].join("\n");
}
