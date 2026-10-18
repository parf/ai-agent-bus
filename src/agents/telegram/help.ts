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
  sub edit <id> [chat=…] [target=…] [filter=…]
  sub delete <id>
  whoami                                the bot's own identity
help lists this; a call may also be JSON: {"method":"post","chat":…}
<chat> is #alias, @alias or a numeric chat id; on post and read it may be
omitted, and then the declared default takes it (README § methods)`;
