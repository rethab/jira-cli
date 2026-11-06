#!/usr/bin/env bash
# Fails when a PR, a local branch or a draft text carries details of the user's own Jira
# instance or of GitHub accounts other than rethab, or contains a file the user handed in
# as private input. The terms are read from local config at run time, so they never have
# to be written into this repository.
#
#   leak-check.sh pr <N>            [--forbid <path>]...   title, body, comments, commits, diff of a PR
#   leak-check.sh branch [<base>]   [--forbid <path>]...   commits and diff of HEAD against <base> (origin/main)
#   leak-check.sh text <file>...    [--forbid <path>]...   drafts (PR body, comment) before they are posted
#
# Extra terms, one per line, can go in ~/.config/api-gap/private-terms.
# Exit status: 0 clean, 1 leak found, 2 usage or setup error.
set -euo pipefail

REPO=rethab/jira-cli
OWN_GH_ACCOUNT=rethab
# Placeholder hosts the codebase already uses in tests and docs.
ALLOWED_SITES='example test demo other your-site your-domain acme company antiklabs'

die() { echo "leak-check: $*" >&2; exit 2; }

mode=${1:-}
[ -n "$mode" ] || die "usage: leak-check.sh pr <N> | branch [<base>] | text <file>... [--forbid <path>]..."
shift

args=()
forbid=()
while [ $# -gt 0 ]; do
	case $1 in
	--forbid) [ $# -ge 2 ] || die "--forbid needs a path"; forbid+=("$2"); shift 2 ;;
	*) args+=("$1"); shift ;;
	esac
done

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
terms=$work/terms
meta=$work/meta
added=$work/added
files=$work/files
for f in "$terms" "$meta" "$added" "$files"; do : >"$f"; done

add_term() {
	local t
	t=$(printf '%s' "$1" | tr -d '\r' | sed -E 's/^[[:space:]]+|[[:space:]]+$//g')
	[ ${#t} -ge 4 ] && printf '%s\n' "$t" >>"$terms"
	return 0
}

cfg=${JIRA_CONFIG_FILE:-$HOME/.config/.jira/.config.yml}
yaml_top() { sed -nE "s/^$1:[[:space:]]*[\"']?([^\"']*)[\"']?[[:space:]]*\$/\\1/p" "$cfg" | head -1; }
yaml_nested() {
	awk -v p="$1" -v k="$2" '
		$0 ~ "^" p ":" { inb = 1; next }
		inb && /^[^[:space:]]/ { inb = 0 }
		inb && $1 == k ":" { sub(/^[^:]*:[[:space:]]*/, ""); gsub(/["\047]/, ""); print; exit }
	' "$cfg"
}

project_key=""
if [ -f "$cfg" ]; then
	server=$(yaml_top server)
	if [ -n "$server" ]; then
		host=${server#*://}; host=${host%%/*}; host=${host%%:*}
		add_term "$host"
		site=${host%%.*}
		case " jira www localhost $ALLOWED_SITES " in *" $site "*) ;; *) add_term "$site" ;; esac
	fi
	login=$(yaml_top login)
	if [ -n "$login" ]; then
		add_term "$login"
		case $login in
		*@*)
			add_term "${login%@*}"
			domain=${login#*@}
			case $domain in
			gmail.com | googlemail.com | outlook.com | hotmail.com | live.com | yahoo.com | icloud.com | me.com | \
				proton.me | protonmail.com | protonmail.ch | pm.me | gmx.* | web.de) ;;
			*) add_term "$domain" ;;
			esac
			;;
		esac
	fi
	project_key=$(yaml_nested project key)
	board=$(yaml_nested board name)
	[ -n "$board" ] && [ ${#board} -ge 6 ] && add_term "$board"
else
	echo "leak-check: no Jira config at $cfg, checking GitHub accounts and extra terms only" >&2
fi

if command -v gh >/dev/null; then
	gh auth status 2>&1 | sed -nE 's/.*account ([^ ]+).*/\1/p' | while read -r acct; do
		[ "$(printf '%s' "$acct" | tr '[:upper:]' '[:lower:]')" = "$OWN_GH_ACCOUNT" ] || add_term "$acct"
	done
fi

extra=$HOME/.config/api-gap/private-terms
[ -f "$extra" ] && while IFS= read -r t; do add_term "$t"; done <"$extra"

sort -u -o "$terms" "$terms"

case $mode in
pr)
	[ ${#args[@]} -eq 1 ] || die "usage: leak-check.sh pr <N>"
	n=${args[0]}
	gh pr view "$n" --repo "$REPO" --json title,body,comments,reviews,commits --jq '
		.title, .body, (.comments[].body), (.reviews[].body),
		(.commits[] | .messageHeadline, .messageBody, (.authors[] | .login, .email, .name))' >"$meta" ||
		die "gh pr view $n failed (try: gh auth switch --user $OWN_GH_ACCOUNT)"
	# Fetch into files first: a failed fetch piped into grep would look like an empty, clean diff.
	gh pr diff "$n" --repo "$REPO" >"$work/diff" || die "gh pr diff $n failed"
	gh pr view "$n" --repo "$REPO" --json files --jq '.files[].path' >"$files" || die "gh pr view $n failed"
	;;
branch)
	base=${args[0]:-origin/main}
	git rev-parse --verify -q "$base" >/dev/null || die "unknown base $base"
	git log "$base..HEAD" --format='%an%n%ae%n%cn%n%ce%n%B' >"$meta" || die "git log failed"
	git diff "$base...HEAD" >"$work/diff" || die "git diff failed"
	git diff --name-only "$base...HEAD" >"$files" || die "git diff --name-only failed"
	;;
text)
	[ ${#args[@]} -ge 1 ] || die "usage: leak-check.sh text <file>..."
	for f in "${args[@]}"; do
		[ -f "$f" ] || die "no such file: $f"
		cat "$f" >>"$added"
	done
	;;
*) die "unknown mode $mode" ;;
esac

if [ -f "$work/diff" ]; then
	grep -E '^\+' "$work/diff" | grep -vE '^\+\+\+ ' >"$added" || true
fi

hits=0
report() { hits=$((hits + 1)); printf 'LEAK (%s): %.200s\n' "$1" "$2"; }

for src in meta added; do
	f=$work/$src
	[ -s "$f" ] || continue
	if [ -s "$terms" ]; then
		while IFS= read -r line; do report "$src: instance or account detail" "$line"; done < <(grep -iF -f "$terms" "$f" || true)
	fi
	if [ -n "$project_key" ]; then
		while IFS= read -r line; do report "$src: issue key of project $project_key" "$line"; done \
			< <(grep -E "(^|[^A-Za-z0-9_])${project_key}-[0-9]+" "$f" || true)
	fi
	while IFS= read -r site; do
		case " $ALLOWED_SITES " in *" $site "*) ;; *) report "$src: real-looking Atlassian site" "${site}.atlassian.net" ;; esac
	done < <(grep -oiE '[a-z0-9-]+\.atlassian\.net' "$f" | tr '[:upper:]' '[:lower:]' | sed 's/\.atlassian\.net$//' | sort -u || true)
done

for p in ${forbid[@]+"${forbid[@]}"}; do
	name=$(basename "$p")
	while IFS= read -r changed; do
		[ "$(basename "$changed")" = "$name" ] && report "private input file committed" "$changed"
	done <"$files"
	# A verbatim copy under another name is as bad as committing the file itself.
	if [ -f "$p" ] && [ -s "$added" ]; then
		grep -vE '^[[:space:]#`*>|-]*$' "$p" | awk 'length >= 40' | sed -E 's/^[[:space:]]+//' >"$work/private-lines" || true
		if [ -s "$work/private-lines" ]; then
			while IFS= read -r line; do report "text copied from $name" "$line"; done \
				< <(sed 's/^+//' "$added" | sed -E 's/^[[:space:]]+//' | grep -xF -f "$work/private-lines" || true)
		fi
	fi
done

if [ $hits -gt 0 ]; then
	echo "leak-check: $hits finding(s); remove them before anything is pushed or posted" >&2
	exit 1
fi
echo "leak-check: clean ($(wc -l <"$terms" | tr -d ' ') private terms checked)"
