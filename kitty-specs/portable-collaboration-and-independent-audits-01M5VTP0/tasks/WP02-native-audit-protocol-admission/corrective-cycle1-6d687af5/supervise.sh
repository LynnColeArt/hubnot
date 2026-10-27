#!/usr/bin/env bash
set -u
cd /home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-b || exit 125
evidence=/home/lynn/projects/nichthub/kitty-specs/portable-collaboration-and-independent-audits-01M5VTP0/tasks/WP02-native-audit-protocol-admission/corrective-cycle1-6d687af5
source_head=$(git rev-parse HEAD)
source_tree=$(git rev-parse HEAD^{tree})
started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"source_head":"%s","source_tree":"%s","supervisor_pid":%s,"started_at":"%s","command":["go","test","-race","-json","./..."],"timeout":"Go default unchanged"}\n' "$source_head" "$source_tree" "$$" "$started" > "$evidence/run.json"
go version > "$evidence/toolchain.txt"
git --version >> "$evidence/toolchain.txt"
go test -race -json ./... > "$evidence/race.jsonl" 2> "$evidence/race.stderr" &
test_pid=$!
printf '%s\n' "$test_pid" > "$evidence/go-test.pid"
wait "$test_pid"
race_exit=$?
final_head=$(git rev-parse HEAD)
finished=$(date -u +%Y-%m-%dT%H:%M:%SZ)
sha256sum "$evidence/race.jsonl" "$evidence/race.stderr" > "$evidence/raw.sha256"
gzip -n -c "$evidence/race.jsonl" > "$evidence/race.jsonl.gz"
sha256sum "$evidence/race.jsonl.gz" > "$evidence/compressed.sha256"
printf '{"source_head":"%s","final_head":"%s","go_test_pid":%s,"exit_code":%s,"finished_at":"%s"}\n' "$source_head" "$final_head" "$test_pid" "$race_exit" "$finished" > "$evidence/exit.json.tmp"
mv "$evidence/exit.json.tmp" "$evidence/exit.json"
exit "$race_exit"
