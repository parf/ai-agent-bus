#!/usr/bin/env python3
"""One install-declared timer attempt; never a bus-call entry point."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import sys

import git_agent as git

# This standalone agent ships beside the repository's existing Python client.
sys.path.insert(0, str(Path(__file__).resolve().parents[3] / 'clients/python'))
from agent_bus import Bus, Refusal


def attempt(repos, repo, config):
    # Only an installed timer invokes this program. A bus body cannot select
    # this mode or grant itself the timer's fixed operate authority.
    os.environ.pop('AB_ROLE_ADMIN', None)
    os.environ['AB_ROLE_OPERATE'] = '1'
    os.environ['AGENT_BUS_CONFIG'] = json.dumps(config)
    try:
        # Automatic pulls refuse any local changes, including ones git might
        # merge around. Never reset, clean, stash, or switch the installed branch.
        result = git.execute(repos, {'method': 'pull', 'repo': repo}, automatic=True)
    except git.GitError as e:
        result = e.result
    except (ValueError, OSError) as e:
        result = {'status': 1, 'error': str(e), 'stdout': '', 'stderr': ''}
    result['at'] = datetime.now(timezone.utc).isoformat()
    path = git.autopull_file(repos[repo]['path'])
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    git.store_autopull(path, result)
    if result['status']:
        print(f"autopull {repo}: {result.get('error', 'pull failed')}\n{result.get('stderr', '')}", file=sys.stderr)
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--repos-json', required=True)
    parser.add_argument('--repo', required=True)
    parser.add_argument('--name', required=True)
    args = parser.parse_args()
    repos = json.loads(args.repos_json)
    if args.repo not in repos: parser.error('undeclared repository')
    try:
        with Bus(args.name) as bus:
            try: config = bus.config
            except Refusal as e:
                if e.status != 404: raise
                config = {}
        return 0 if attempt(repos, args.repo, config or {})['status'] == 0 else 1
    except Exception as e:
        # Connection diagnostics can contain credentials; name the failure's
        # class only. A failure reading config must not pull with default keys.
        result = {'at': datetime.now(timezone.utc).isoformat(), 'status': 1,
                  'error': 'configuration unavailable: ' + type(e).__name__}
        path = git.autopull_file(repos[args.repo]['path'])
        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        git.store_autopull(path, result)
        print(result['error'], file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
