#!/usr/bin/env bash
# Create a worktree for one agent and one task. See docs/agents/worktrees.md.
#
#   scripts/wt-new.sh codex  T-001 prototype          # feat/T-001-prototype from origin/main
#   scripts/wt-new.sh codex  BUG-004 hold-timer       # fix/BUG-004-hold-timer from origin/main
#   scripts/wt-new.sh gemini T-001 prototype          # test/T-001-prototype from origin/feat/T-001-prototype
set -euo pipefail

agent=${1:?usage: wt-new.sh <codex|gemini> <T-xxx|BUG-xxx> <slug>}
task=${2:?missing task id}
slug=${3:?missing slug}

root=$(git rev-parse --show-toplevel)
wt="$root/../wt/$agent-$task"

case "$agent" in
  codex)
    port=8081
    if [[ $task == BUG-* ]]; then branch="fix/$task-$slug"; else branch="feat/$task-$slug"; fi
    base=origin/main
    ;;
  gemini)
    port=8082
    branch="test/$task-$slug"
    base="origin/feat/$task-$slug"
    ;;
  *) echo "agent must be codex or gemini" >&2; exit 1 ;;
esac
db="dropkilat_$agent"

[[ -e $wt ]] && { echo "already exists: $wt" >&2; exit 1; }

git -C "$root" fetch -q origin
git -C "$root" rev-parse -q --verify "$base" >/dev/null || { echo "base $base not found on origin" >&2; exit 1; }
git -C "$root" worktree add -b "$branch" "$wt" "$base"

# .env is gitignored, so every worktree needs its own copy.
src="$root/.env"; [[ -f $src ]] || src="$root/.env.example"
sed -e "s|^DK_HTTP_ADDR=.*|DK_HTTP_ADDR=:$port|" \
    -e "s|\(DK_DATABASE_URL=.*/\)dropkilat?|\1$db?|" \
    -e "s|\(DK_MIGRATE_DATABASE_URL=.*/\)dropkilat?|\1$db?|" \
    "$src" > "$wt/.env"

# Create the agent's database if the dev container is running; harmless if it already exists.
if docker compose -f "$root/deploy/docker-compose.dev.yml" ps --status running db >/dev/null 2>&1; then
  docker compose -f "$root/deploy/docker-compose.dev.yml" exec -T db psql -q -U dropkilat -d dropkilat <<SQL || true
SELECT 'CREATE DATABASE $db OWNER dropkilat' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
GRANT CONNECT ON DATABASE $db TO dropkilat_app;
SQL
else
  echo "note: dev db not running; run 'make db-up' then re-run the CREATE DATABASE step for $db" >&2
fi

echo
echo "worktree: $wt"
echo "branch:   $branch (from $base)"
echo "server:   http://localhost:$port   db: $db"
