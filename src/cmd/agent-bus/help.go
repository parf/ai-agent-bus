package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"golang.org/x/sys/unix"
)

func agentHelp(args []string, out *os.File) error {
	pos, flags, err := split(args)
	if err := only(flags, err, "no-color", "wait"); err != nil {
		return err
	}
	if len(pos) == 0 {
		_, err := fmt.Fprintln(out, usage)
		return err
	}
	if len(pos) > 2 {
		return fmt.Errorf("help wants <agent> and optionally config, secret or json")
	}
	if !strings.HasPrefix(pos[0], "#") {
		if len(pos) != 1 {
			return fmt.Errorf("help wants one command, or a #agent and optional form")
		}
		text := verbUsage(pos[0])
		if text == "" {
			return fmt.Errorf("unknown command %q", pos[0])
		}
		_, err := fmt.Fprintln(out, text)
		return err
	}
	ask := protocol.AskHelp
	if len(pos) == 2 {
		switch pos[1] {
		case "config":
			ask = protocol.AskHelpConfig
		case "secret":
			ask = protocol.AskHelpSecret
		case "json":
			ask = protocol.AskHelpJSON
		default:
			return fmt.Errorf("unknown help form %q: use config, secret or json", pos[1])
		}
	}
	wait, err := callWait(flags["wait"])
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if err := callAddress(ctx); err != nil {
		return plainHelpError(err)
	}
	raw, err := callTarget(ctx, protocol.Envelope{To: pos[0], Topic: "help", Tag: newTag(), Ask: ask}, wait, io.Discard)
	if err != nil {
		return plainHelpError(err)
	}
	var message protocol.Envelope
	if err := json.Unmarshal(raw, &message); err != nil {
		return fmt.Errorf("invalid help answer: %w", err)
	}
	text := humanText(message.Body)
	if ask == protocol.AskHelpJSON {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(text), "", "  "); err != nil {
			return fmt.Errorf("agent's help is not JSON: %w", err)
		}
		text = pretty.String()
	}
	if helpColour(out, has(flags, "no-color")) {
		if ask == protocol.AskHelpJSON {
			text = colourHelpJSON(text)
		} else {
			text = colourHelpText(text)
		}
	}
	_, err = fmt.Fprintln(out, strings.TrimRight(text, "\n"))
	return err
}

func plainHelpError(err error) error {
	var reply struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(err.Error()), &reply) == nil && reply.Error != "" {
		return fmt.Errorf("%s", reply.Error)
	}
	return err
}

func helpColour(out *os.File, disabled bool) bool {
	_, noColour := os.LookupEnv("NO_COLOR")
	if disabled || noColour {
		return false
	}
	_, err := unix.IoctlGetTermios(int(out.Fd()), unix.TCGETS)
	return err == nil
}

func paint(code, text string) string { return "\x1b[" + code + "m" + text + "\x1b[0m" }

var helpRole = regexp.MustCompile(`role: [^;\n]+`)

func colourHelpText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "  ") || line == "" {
			continue
		}
		line = helpRole.ReplaceAllStringFunc(line, func(role string) string { return paint("33", role) })
		line = strings.ReplaceAll(line, "; destructive", "; "+paint("31", "destructive"))
		line = strings.ReplaceAll(line, "(required)", paint("1;31", "(required)"))
		if at := strings.IndexAny(line, " :"); at > 0 {
			line = paint("1", line[:at]) + line[at:]
		} else {
			line = paint("1", line)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

var jsonHelpToken = regexp.MustCompile(`"(?:\\.|[^"\\])*"|\b(?:true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?`)

func colourHelpJSON(text string) string {
	var out strings.Builder
	previous := 0
	for _, span := range jsonHelpToken.FindAllStringIndex(text, -1) {
		out.WriteString(text[previous:span[0]])
		token := text[span[0]:span[1]]
		code := "36" // numbers
		if token[0] == '"' {
			code = "32"
			if strings.HasPrefix(strings.TrimLeft(text[span[1]:], " \n\r\t"), ":") {
				code = "1;34"
			}
		} else if token == "true" || token == "false" {
			code = "35"
		} else if token == "null" {
			code = "90"
		}
		out.WriteString(paint(code, token))
		previous = span[1]
	}
	out.WriteString(text[previous:])
	return out.String()
}
