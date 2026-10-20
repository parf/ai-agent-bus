package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployCheckPrintsOnlyBodyAndDefaultsToLocalRealm(t *testing.T) {
	script, err := filepath.Abs("../../check-deploy.sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"hostname": "#!/bin/sh\nprintf fixture.example",
		"agent-bus": `#!/bin/sh
if [ "$1" = status ]; then printf '%s' "$STATUS"; exit 0; fi
printf '%s' "$2" > "$TARGET_FILE"
printf 'ack chatter\n' >&2
printf '%s' "$ANSWER"
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		answer string
		status string
		args   []string
		want   string
		ok     bool
	}{
		{`{"body":"✅ one\n✅ two\n✅ three\n✅ four\n✅ five\n✅ six","from":"hidden-envelope"}`, `{"you":"tester@bus.example"}`, nil, "#debug@bus.example", true},
		{"✅ one\n❌ failed", "", []string{"#debug@other"}, "#debug@other", false},
		{`{"body":12}`, "invalid status", nil, "#debug@fixture.example", false},
	} {
		cmd := exec.Command(script, tc.args...)
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "STATUS="+tc.status, "ANSWER="+tc.answer, "TARGET_FILE="+filepath.Join(dir, "target"))
		out, err := cmd.CombinedOutput()
		if (err == nil) != tc.ok {
			t.Fatalf("exit %v output %s", err, out)
		}
		target, _ := os.ReadFile(filepath.Join(dir, "target"))
		if string(target) != tc.want {
			t.Fatalf("target %q", target)
		}
		if strings.Contains(string(out), "ack chatter") || strings.Contains(string(out), "hidden-envelope") || strings.Contains(string(out), `"body"`) {
			t.Fatalf("printed envelope: %s", out)
		}
		if tc.ok && strings.Count(string(out), "✅") != 6 {
			t.Fatalf("missing body: %s", out)
		}
	}
}
