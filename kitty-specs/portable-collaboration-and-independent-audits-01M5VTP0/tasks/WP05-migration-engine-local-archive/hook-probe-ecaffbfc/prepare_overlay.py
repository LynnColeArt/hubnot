#!/usr/bin/env python3
"""Prepare the preserved independent review probe without changing product files.
Usage: python3 prepare_overlay.py WORKTREE SCRATCH_DIRECTORY
Then use the printed go test command from WORKTREE. Expected exit at ecaffbfc: 1.
"""
import json, pathlib, shlex, sys
worktree=pathlib.Path(sys.argv[1]).resolve()
scratch=pathlib.Path(sys.argv[2]).resolve()
scratch.mkdir(parents=True,exist_ok=True)
source=(pathlib.Path(__file__).parent/'migration-hook-probe.go.txt').read_text()
source=source.replace('/tmp/wp05-review-ecaffbfc', str(scratch))
probe=scratch/'migration_hook_review_test.go'
probe.write_text(source)
overlay=scratch/'overlay.json'
overlay.write_text(json.dumps({'Replace':{str(worktree/'migration_hook_review_test.go'):str(probe)}}))
print('go test -race -overlay '+shlex.quote(str(overlay))+" -run '^TestReviewWP05DestinationRelativeHookIsolation$' -count=1 -v .")
