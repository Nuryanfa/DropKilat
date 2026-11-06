#!/usr/bin/env bash
# Tech-lead dashboard: what every agent worktree is doing. See docs/agents/worktrees.md.
set -uo pipefail

root=$(git rev-parse --show-toplevel)
now=$(date +%s)
stale_after=$((24 * 3600))
git -C "$root" fetch -q origin 2>/dev/null || echo "warning: fetch failed, origin data may be old"

# Paths an agent may change without explanation (extended regex on repo-relative paths).
allowed_for() {
  case "$1" in
    codex)  echo '^(cmd/|internal/|web/|migrations/|prototype/|tests/|go\.(mod|sum)$|sqlc\.yaml$|\.env\.example$|docs/tasks/T-[0-9]+\.md$|docs/(backend|database-schema|app-flow|design)\.md$)' ;;
    gemini) echo '(_test\.go$|^tests/|^docs/test-plan\.md$|^docs/tasks/)' ;;
    *)      echo '.' ;;
  esac
}

declare -A wt_tasks=()

echo "== Worktrees"
while read -r line; do
  case "$line" in
    worktree\ *) path=${line#worktree } ;;
    branch\ *)   branch=${line#branch refs/heads/} ;;
    detached)    branch="(detached)" ;;
    "")
      [[ -z ${path:-} ]] && continue
      name=$(basename "$path")
      if [[ $path == "$root" ]]; then agent=claude; else agent=${name%%-*}; fi
      task=$(grep -oE '(T|BUG)-[0-9]+' <<<"$name$branch" | head -1)
      [[ -n $task ]] && wt_tasks[$task]=1

      last=$(git -C "$path" log -1 --format='%ct|%cr|%s' 2>/dev/null)
      ts=${last%%|*}; rest=${last#*|}; age=${rest%%|*}; subject=${rest#*|}
      dirty=$(git -C "$path" status --porcelain | wc -l | tr -d ' ')
      read -r behind ahead < <(git -C "$path" rev-list --left-right --count origin/main...HEAD 2>/dev/null || echo "? ?")

      echo
      echo "[$agent] $name  ($branch)"
      echo "  last commit: $age — $subject"
      echo "  ahead $ahead / behind $behind origin/main · uncommitted files: $dirty"
      [[ $agent != claude && -n $ts && $((now - ts)) -gt $stale_after ]] && echo "  ! no commit for more than 24h"
      [[ $behind != "?" && $behind -gt 20 ]] && echo "  ! more than 20 commits behind main: ask for a rebase"

      if [[ $agent != claude ]]; then
        changed=$( { git -C "$path" diff --name-only origin/main...HEAD; git -C "$path" status --porcelain | cut -c4-; } 2>/dev/null | sort -u)
        outside=$(grep -vE "$(allowed_for "$agent")" <<<"$changed" | grep -v '^$')
        [[ -n $outside ]] && { echo "  ! outside $agent's area:"; sed 's/^/      /' <<<"$outside"; }
      fi
      path=""; branch=""
      ;;
  esac
done < <(git -C "$root" worktree list --porcelain; echo)

echo
echo "== Task cards"
printf '%-9s %-8s %-12s %s\n' ID OWNER STATUS TITLE
for card in "$root"/docs/tasks/{T,BUG}-*.md; do
  [[ -f $card && $card != *-qa.md ]] || continue
  id=$(basename "$card" .md)
  field() { sed -n "s/^$1: *\([^#]*\).*/\1/p" "$card" | head -1 | sed 's/ *$//'; }
  status=$(field status)
  printf '%-9s %-8s %-12s %s\n' "$id" "$(field owner)" "$status" "$(field title)"
  [[ $status =~ ^(in-progress|in-review)$ && -z ${wt_tasks[$id]:-} ]] && echo "          ! $status but no worktree"
  grep -qE '^## Questions' "$card" && awk '/^## Questions/{f=1;next} /^## /{f=0} f && /\?/ && !/^\(/' "$card" | grep -q . \
    && echo "          ? has questions — check they are answered"
done
for t in "${!wt_tasks[@]}"; do
  [[ -f $root/docs/tasks/$t.md ]] || echo "! worktree for $t but no card docs/tasks/$t.md"
done
