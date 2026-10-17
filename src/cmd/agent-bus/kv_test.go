package main

import (
	"strings"
	"testing"
)

// A piped integer ends in a newline, as a piped JSON value does.
func TestAPipedIntKeepsItsNewline(t *testing.T) {
	body := map[string]any{}
	if err := kvValue(body, "int", []byte("5\n")); err != nil || body["value"] != int64(5) {
		t.Fatalf("echo 5 | kv set … --int: %v, %v", body["value"], err)
	}
	if err := kvValue(body, "int", []byte("5 x")); err == nil {
		t.Fatal("a non-integer was taken")
	}
	// A string is not trimmed.
	if err := kvValue(body, "string", []byte("a\n")); err != nil || body["value"] != "a\n" {
		t.Fatalf("a string lost its newline: %q, %v", body["value"], err)
	}
}

// --force takes no value, wherever it stands.
func TestForceEatsNoWord(t *testing.T) {
	pos, flags := split(strings.Fields("ops@h --force deploy"))
	if _, ok := flags["force"]; !ok || len(pos) != 2 || pos[1] != "deploy" {
		t.Fatalf("release ops@h --force deploy split as %v %v", pos, flags)
	}
}
