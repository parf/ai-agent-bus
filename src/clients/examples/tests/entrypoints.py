"""Exercise the worker/caller process boundary with inherited session credentials."""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile

examples = Path(__file__).resolve().parents[1]
repo = examples.parents[2]
with tempfile.TemporaryDirectory(dir=repo / "tmp", prefix="example-entrypoints-") as directory:
    work = Path(directory)
    (work / "bin").mkdir()
    (work / "tokens").mkdir()
    probe = work / "probe"
    probe.write_text("#!/bin/sh\nexec " + shlex.quote(sys.executable) + " -c " + shlex.quote(
        'import os,json; print(json.dumps({"name":os.environ["AGENT_BUS_NAME"],"token":os.environ["AGENT_BUS_TOKEN"]}))') + "\n")
    probe.chmod(0o700)
    (work / "bin" / "python3").symlink_to(probe)
    compiler = work / "bin" / "go"
    compiler.write_text('#!/bin/sh\nset -eu\n[ "$1" = build ] && [ "$2" = -o ]\ncp "$ENTRYPOINT_PROBE" "$3"\nchmod +x "$3"\n')
    compiler.chmod(0o700)
    env = {**os.environ, "PATH": str(work / "bin") + os.pathsep + os.environ["PATH"],
           "AGENT_BUS_NAME": "#caller-session@localhost", "AGENT_BUS_TOKEN": "caller-token",
           "EXAMPLES_TOKENS": str(work / "tokens"), "EXAMPLES_ROOT": str(repo),
           "STATS_ROOT": str(work), "DEMO_FILE": str(work / "sample.json"),
           "ENTRYPOINT_PROBE": str(probe), "AGENT_BUS_EXAMPLE_BUILD_DIR": str(work / "build")}
    for language, option in [("python", "AB_PYTHON_WORKER_LAUNCHER"), ("go", "AB_GO_WORKER_LAUNCHER")]:
        name = f"#stats-{language}@localhost"
        token_file = work / "tokens" / f"_stats-{language}_localhost"
        token_file.write_text(language + "-token\n")
        launcher = Path(os.environ.get(option, str(examples / language / "run.sh")))
        run = subprocess.run(["bash", str(launcher)], env=env, capture_output=True, text=True, timeout=10)
        assert run.returncode == 0, run.stderr
        assert json.loads(run.stdout) == {"name": name, "token": language + "-token"}, f"{language} worker must reject inherited caller identity"
        demo = subprocess.run(["bash", str(examples / language / "demo.sh")], env=env, capture_output=True, text=True, timeout=10)
        assert demo.returncode == 0, demo.stderr
        assert json.loads(demo.stdout) == {"name": env["AGENT_BUS_NAME"], "token": env["AGENT_BUS_TOKEN"]}, f"{language} demo must preserve caller identity"
        token_file.unlink()
        missing = subprocess.run(["bash", str(launcher)], env=env, capture_output=True, text=True, timeout=10)
        assert missing.returncode != 0 and not missing.stdout, f"{language} worker must fail when its own token is missing"
        print(f"{language} worker and demo identities verified")
