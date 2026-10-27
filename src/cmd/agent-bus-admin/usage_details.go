package main

var adminDetails = map[string]struct{ description, examples string }{
	"user":              {"Manage people who use this node. <name> is a bus principal, <account> is a local Linux account, and <key.pub|-> is a public SSH key file or stdin. A key reaches one forced command and no shell: token retrieval normally, or administration when --admin is granted.", "agent-bus-admin user list\nagent-bus-admin user add bob@demo ./bob.pub"},
	"user add":          {"<name> is a bus principal; <key.pub|-> is its public SSH key file, or - for stdin. Create the user if needed and install a forced SSH command. Without --admin the key may only retrieve that principal's credential; --admin also grants administration, never a shell.", "agent-bus-admin user add bob@demo ./bob.pub"},
	"user import-local": {"<name> is an existing bus user; <account> is a Linux account. Import its human-readable name from the local account database, keeping any person name already set.", "agent-bus-admin user import-local bob@demo bob"},
	"user add-local":    {"<name> is the new bus user; <account> is a local Linux account. Add a keyless user whose login socket belongs to that account.", "agent-bus-admin user add-local bob@demo bob"},
	"user list":         {"List enrolled bus users. No positional arguments.", "agent-bus-admin user list"},
	"user remove":       {"<name> is the bus user whose SSH access is removed. Other users' keys remain.", "agent-bus-admin user remove bob@demo"},
	"account":           {"Read or change the map from Linux accounts to bus users. <account> is a local Linux account and <name> is a bus principal. A change may require a daemon restart for login sockets.", "agent-bus-admin account list\nagent-bus-admin account set bob bob@demo"},
	"account list":      {"Show local account-to-principal mappings and whether a daemon restart is needed. No positional arguments.", "agent-bus-admin account list"},
	"account set":       {"<account> is a Linux account; <name> is its bus principal. Set the login-socket mapping and report whether a daemon restart is needed.", "agent-bus-admin account set bob bob@demo"},
	"account remove":    {"<account> is the local Linux account whose login mapping is removed. Report whether a daemon restart is needed.", "agent-bus-admin account remove bob"},
	"token":             {"<name> is the principal whose credential is requested. Use the shared token command; without --rotate, asking again reads the same credential. An SSH forced command cannot request another principal. --fingerprint prints the public TLS certificate pin.", "agent-bus-admin token bob@demo > private.token\nagent-bus-admin token --fingerprint"},
	"tls":               {"Show the served TLS certificate's SHA-256 fingerprint, names and expiry. This public information needs no account change. No positional arguments.", "agent-bus-admin tls"},
}
