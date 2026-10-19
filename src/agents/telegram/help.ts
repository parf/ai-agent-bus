// The help text, its own module so the test can hold the parser and the
// help to the same method set (tg1-review P3-2): what help lists, the
// parser accepts; what the parser accepts, help lists.
export const help = `telegram: a Telegram bot on the bus
  post [<chat>] <text…>                 post; answers chat and message id
  read [<chat>] [N]                     the last N messages the bot received
  channels [filter]                     the chats the bot has seen
  channel <chat>                        one chat
  users [filter]                        the senders the bot has seen
  user <@alias|id>                      one sender
  react <chat> <id> <emoji>             add a reaction
  edit <chat> <id> <text…>              edit the bot's own message
  delete <chat> <id>                    delete the bot's own message
  sub add <chat> <target> [filter]      forward a chat's messages
  sub list                              the subscriptions
  sub edit <id> [channel=…] [target=…] [filter=…]
  sub delete <id>
  refused                               last 20 unlisted senders; Owner/Maintainers only
  whoami                                the bot's own identity
help lists this; a call may also be JSON: {"method":"post","channel":…}
<chat> is #alias, @alias or a numeric chat id.
Posts end with a permanent session #tag; listed Telegram users may Reply or
write #tag text (full bus names also work). Unmatched text follows subscriptions,
then Configuration.route, or gets a not-understood reply listing tags.
On post and read the chat may be omitted, and then the declared default takes it (README § methods)`;
