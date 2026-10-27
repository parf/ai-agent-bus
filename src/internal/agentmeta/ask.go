package agentmeta

// AskHandler answers only self-description requests, never ordinary work.
type AskHandler func() (string, error)

// DispatchAsk returns false only for ordinary input. Unknown asks are replies.
func DispatchAsk(ask string, handlers map[string]AskHandler) (string, bool, error) {
	if ask == "" {
		return "", false, nil
	}
	if answer := handlers[ask]; answer != nil {
		body, err := answer()
		return body, true, err
	}
	return "unsupported: " + ask, true, nil
}

// AskHandlers supplies authored metadata unless the package implements native help.
func (m Manifest) AskHandlers(dir string, roles []string, target string) map[string]AskHandler {
	handlers := map[string]AskHandler{}
	if m.Help == "native" {
		return handlers
	}
	handlers["mcp"] = func() (string, error) { return `{"tools":[]}`, nil }
	for _, ask := range []string{"help", "help-config", "help-secret", "help-json"} {
		handlers[ask] = func() (string, error) { return m.Answer(dir, ask, roles, false, target) }
	}
	return handlers
}
