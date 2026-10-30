package main

import (
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/version"
	"strings"
)

type commandHelp struct{ name, synopsis, description, options string }

var commandGroups = []struct {
	title    string
	commands []commandHelp
}{
	{"MAIN COMMANDS", []commandHelp{
		{"status", "status", "Show bus status and your name", ""},
		{"ls", "ls [<name>]", "List visible records (readers by default)", "--kind=user|agent|queue|pubsub|service|resource  List every visible record of this kind\n--tag=tag  Filter by tag\n--all  Include every visible kind\n-h, --table  Print a human-readable table (default JSON)"},
		{"send", "send <to> [<text>|-]", "Send once without waiting", "--topic=topic  Message topic\n--tag=tag  Correlation tag\n--reply-to=name  Reply inbox\n--ttl=duration  Lifetime (default recipient retention; shorter wins)\n--help  Ask the destination for help (with <to>)\n--help-config  Ask for Configuration declarations\n--help-secret  Ask for environment declarations\n--help-json  Ask for machine metadata\n--status  Ask for agent status\n--mcp  Ask for MCP tools"},
		{"call", "call <to> [<text>|-]\n  agent-bus call --tagged=<tag> <body>", "Send once and wait for the answer", "--topic=topic  Message topic\n--tag=tag  Correlation tag\n--tagged=tag  Call every visible matching agent\n--wait=duration  Answer deadline (default 30s)\n--help  Ask the destination for help (with <to>)\n--help-config  Ask for Configuration declarations\n--help-secret  Ask for environment declarations\n--help-json  Ask for machine metadata\n--status  Ask for agent status\n--mcp  Ask for MCP tools"},
		{"consume", "consume", "Read messages from your inbox", "--inbox=name  Inbox to read\n--topic=topic  Match topic\n--tag=tag  Match tag\n--wait=duration  Long-poll deadline (default 30s)\n--follow  Keep reading\n--share  Read as one of several workers"},
		{"reply", "reply <message-id> <text>\n  agent-bus reply --to=<name> <text>", "Answer a message", "--to=name  Reply destination\n--topic=topic  Original topic\n--tag=tag  Original correlation tag"},
		{"run", "run <name> <script>\n  agent-bus run [-N] -", "Run a script as an agent", "--algo=args|json  Script input form (default args)\n-N  Run up to N messages concurrently (e.g. -4)\n--descr=text  Description\n--allow=names|*|@owner  Comma-separated access terms (comma-separated, * or @owner)\n--personal  Personal agent\n--tags=tags  Comma-separated tags\n--sandbox=on|off  Confinement (default off)\n--network  Allow sandbox network (default denied when confined)\n--package-dir=path  Package metadata for asks\n--roles=roles  Understood roles\n--stay-registered  Keep record and credential at graceful stop\n--no-asks  Answer unsupported asks without launching the script"},
	}},
	{"MESSAGES", []commandHelp{
		{"ack", "ack <message-id>", "Say you received a message", ""},
		{"done", "done <message-id>", "Say a task is done", ""},
		{"publish", "publish --channel=<name> <text>", "Publish to a channel", "--channel=name  Destination channel"},
		{"unsubscribe", "unsubscribe <channel>", "Stop receiving a channel's copies", ""},
	}},
	{"AGENTS AND RECORDS", []commandHelp{
		{"register", "register <name>", "Register an agent, channel, service or resource", "--kind=user|agent|queue|pubsub|service|resource  Record kind (default from name; otherwise service)\n--service-addr=address  External service address\n--protocol=protocol  External service protocol\n--descr=text  Description\n--personal  Personal agent\n--tags=tags  Comma-separated tags\n--allow=names|*|@owner  Comma-separated access terms\n--overflow=ring|strict  Drop oldest or refuse (default strict)\n--ttl=duration  Retention (default no expiry)\n--bound=count  Capacity (default 1000)\n--uri=uri  Resource URI\n--template  URI is a template\n--source=name  Source agent or MCP service\n--mime=type  Media type\n--title=text  Resource title\n--size=bytes  Resource size\n--mcp-name=name  MCP resource name\n--icons=json  Icons\n--annotations=json  Annotations"},
		{"unregister", "unregister <name>", "Remove an idle record", ""},
		{"manage", "manage <name>", "Change a record and who may use it", "--descr=text  Description\n--status=active|inactive  State\n--owner=user  Transfer ownership\n--deliver-to=name  Route destination (empty clears)\n--roles=roles  Understood roles (empty clears)\n--tags=tags  Replace tags\n--add-tags=names  Append tags\n--add-to-set-tags=names  Add without duplicates tags\n--remove-tags=names  Remove tags\n--add-allow=names  Append allow\n--add-to-set-allow=names  Add without duplicates allow\n--remove-allow=names  Remove allow\n--add-maintainers=names  Append maintainers\n--add-to-set-maintainers=names  Add without duplicates maintainers\n--remove-maintainers=names  Remove maintainers\n--add-deliver-to=names  Append deliver-to\n--add-to-set-deliver-to=names  Add without duplicates deliver-to\n--remove-deliver-to=names  Remove deliver-to"},
		{"may-send", "may-send <name>", "Check you may send, without sending", ""},
		{"stop", "stop <name>", "Stop a script agent on this host", ""},
		{"logs", "logs <name>", "Read a local script agent's output", "--lines=count  Last lines (default 50)\n--follow  Follow new output"},
		{"config", "config <name> [<json>|-]", "Read or set a record's Configuration", ""},
		{"secret", "secret <name> [<bytes>|-]", "Read or set a record's Secret", ""},
		{"group", "group <@name> [<member> ...]", "See or set who is in a group", ""},
	}},
	{"STORAGE AND LOCKS", []commandHelp{
		{"kv", "kv get <record> <key>\n  agent-bus kv set <record> <key> <value|->\n  agent-bus kv delete <record> <key>\n  agent-bus kv inc <record> <key> [n]\n  agent-bus kv json <record> <key> <ops>", "Read or change stored values", "--int  Integer storage (get,set,delete)\n--json  JSON storage (get,set,delete)\n--add  Set only if absent (set only)\n--replace  Set only if present (set only)"},
		{"lock", "lock <record> <name>", "Wait for and take a named lock", "--ttl=duration  Lease lifetime (default 30s)\n--wait=duration  Acquisition deadline (default 30s)"},
		{"try-lock", "try-lock <record> <name>", "Take a named lock or refuse immediately", "--ttl=duration  Lease lifetime (default 30s)"},
		{"release", "release <record> <name>", "Release a named lock", "--force  Release another holder (manager only)"},
		{"extend", "extend <record> <name>", "Keep your lock longer", "--ttl=duration  New lease lifetime (default 30s)"},
		{"holders", "holders <record>", "Show who holds locks", ""},
	}},
	{"CREDENTIALS", []commandHelp{
		{"token", "token [<name>]", "Get your credential", "-h, --help  Show this command help\n--key=file  SSH private key (default $HOME/.ssh/id_ed25519)\n--fingerprint  Print the TLS pin instead of a credential\n--rotate  Rotate the credential"},
		{"enrol", "enrol <user[@realm]>", "Join with a published SSH key", "--key=file  SSH private key (default $HOME/.ssh/id_ed25519)"},
	}},
}

const globalOptions = "\n\nGLOBAL OPTIONS\n  --addr=<address>  Daemon address; place before the command\n  --agent=<name>  Expand a name to #name (name shorthand)\n  -h, --help  Overview when used alone; <command> --help shows its page\n  --version  Print version and build; use alone\n  Use -h before a command for overview help; ls -h requests its table."

func commandPage(text string) string {
	return display.ManText(strings.Replace(text, "\n\nEXAMPLES", globalOptions+"\n\nEXAMPLES", 1))
}

var usage = generalHelp()

func generalHelp() string {
	var b strings.Builder
	b.WriteString("agent-bus " + version.String + " — talk to agent-busd · " + display.Homepage + "\n\nUSAGE\n  agent-bus [--addr=<address>] <command> [<args>]\n  agent-bus help <command> · help '#agent' · --version | -h, --help\n\nDESCRIPTION\n  Talk to the bus: send messages, list records and run agents.\n")
	for _, g := range commandGroups {
		b.WriteString("\n" + g.title + "\n")
		for _, c := range g.commands {
			synopsis := strings.Split(c.synopsis, "\n")[0]
			if c.name == "kv" {
				synopsis = "kv get|set|delete|inc|json"
			}
			if c.name == "ls" {
				synopsis = "ls [-h] [--all] [<name>]"
			}
			if c.name == "send" || c.name == "call" {
				synopsis = c.name + " <to> <text>"
			}
			b.WriteString("  " + synopsis + " — " + c.description + "\n")
		}
	}
	b.WriteString(globalOptions)
	b.WriteString("\n\nNAMES\n  Quote #agent names: the shell treats # as a comment.\n  --agent <name> expands an agent name wherever a name is accepted.\n\nENVIRONMENT\n  AGENT_BUS_ADDR — daemon address (or automatic local socket discovery)\n  AGENT_BUS_TOKEN — caller credential; unnecessary on your login socket\n  AGENT_BUS_TLS_FINGERPRINT — HTTPS certificate pin\n  NO_COLOR — disable help colour\n\nEXIT STATUS\n  0 — success\n  1 — invalid input, refusal, transport failure or deadline exceeded\n\nSEE ALSO\n  " + display.SourceURL + "/docs/user/cli.md\n  " + display.Source)
	return display.ManText(b.String())
}
func verbUsage(verb string) string {
	switch verb {
	case "enroll":
		verb = "enrol"
	case "agent-template":
		verb = "config"
	}
	if verb == "help" {
		return commandPage("USAGE\n  agent-bus help <command>\n  agent-bus help '#agent' [config|secret|json]\n\nDESCRIPTION\n  Learn how to use a command, or ask an agent what it can do. <command> names a CLI command. A #agent answers with its own help; config, secret or json selects another form. Agent help uses colour only on a terminal, unless NO_COLOR or --no-color is set. Refusals go to stderr with a nonzero exit.\n\nOPTIONS\n  --no-color  Disable terminal colour\n  --wait=duration  Agent answer deadline (default 30s)\n\nEXAMPLES\n  agent-bus help ls\n  agent-bus help '#debug@demo' json\n\nSEE ALSO\n  " + display.SourceURL + "/docs/user/cli.md\n  " + display.Source)
	}
	for _, g := range commandGroups {
		for _, c := range g.commands {
			if c.name == verb {
				detail := commandDetails[verb]
				out := "USAGE\n  agent-bus " + c.synopsis + "\n\nDESCRIPTION\n  " + detail.description
				if c.options == "" {
					out += "\n\nOPTIONS\n  None."
				} else {
					out += "\n\nOPTIONS\n  " + strings.ReplaceAll(c.options, "\n", "\n  ")
				}
				out += "\n\nEXAMPLES\n  " + strings.ReplaceAll(detail.examples, "\n", "\n  ")
				links := display.SourceURL + "/docs/user/cli.md"
				if verb == "run" {
					links += "\n  " + display.SourceURL + "/docs/user/dedicated-runner.md"
				}
				return commandPage(out + "\n\nSEE ALSO\n  " + links + "\n  " + display.Source)
			}
		}
	}
	return ""
}
