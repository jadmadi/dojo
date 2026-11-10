# Release checklist

Run every gate before a tag. A failed gate blocks the release.

| Gate | Runs |
|---|---|
| Static, tests, version | CI, blocking |
| Smoke, promise, changelog | Local, blocking |
| Publish, post-release | Manual, blocking |

## 1. Static gate

```bash
gofmt -l .
go vet ./...
go build ./...
```

PASS: no files listed, no diagnostics, exit 0.

## 2. Test gate

```bash
go test -race ./...
go run ./cmd/dojo doctor --skills --security
```

PASS: every package ok, doctor reports 0 warnings.

## 3. Smoke gate

Run the binary in a throwaway HOME, never your real config.

```bash
export HOME=$(mktemp -d)
export XDG_CONFIG_HOME=$HOME/.config XDG_CACHE_HOME=$HOME/.cache
go build -o /tmp/dojo-smoke-bin ./cmd/dojo
d=/tmp/dojo-smoke-bin

mkdir -p /tmp/dojo-smoke-ws && cd /tmp/dojo-smoke-ws
$d init smoke-skill --description "Release smoke"
$d toc smoke-skill --local | head -1
$d load smoke-skill --local -s when-to-use | head -1
$d list --json | jq -e 'length > 0'
$d status --raw
$d doctor --skills --security
$d sync --one-way -t all
$d mcp --print-schema | jq -e '.tools | length == 8'
$d completion bash | head -1
```

PASS: every command exits 0 and prints content. A silent empty result fails.

## 4. Promise gate

Read the release diff and answer three questions for every changed flag, file
format, or behaviour:

- Does README document it?
- Does `skills/dojo/SKILL.md` describe it?
- Does `project.jsonc` list it as a feature if it is user-visible?

No unmeasured metric ships. If the docs state a percentage, the release must
include the measurement that produced it.

PASS: every documented flag exists, every claim has evidence.

## 5. Version gate

```bash
v=$(sed -n 's/.*DefaultVersion = "\(.*\)"/\1/p' internal/version/version.go)
sed -n 's/.*release-v\([0-9.]*\)-green.*/README \1/p' README.md
sed -n 's/.*"version": "\([0-9.]*\)".*/project.jsonc \1/p' project.jsonc | head -1
sed -n 's/^  version: "\([0-9.]*\)"/SKILL.md \1/p' skills/dojo/SKILL.md
```

PASS: all four values equal. CI enforces this.

## 6. Changelog gate

`CHANGELOG.md` has a dated entry for the version, grouped as Added, Changed,
Fixed, and describing user-visible effects.

PASS: the tag date and the changelog date match.

## 7. Publish gate

```bash
git tag -a vX.Y.Z -m "..."
git push origin vX.Y.Z
gh run watch <run-id> --exit-status
gh release view vX.Y.Z --json assets --jq '.assets | length'
```

PASS: workflow succeeds, 7 assets published.

## 8. Post-release gate

```bash
HOME=$(mktemp -d) sh -c \
  'curl -fsSL https://raw.githubusercontent.com/jadmadi/dojo/main/scripts/install.sh | sh'
$HOME/.local/bin/dojo --version
```

PASS: the installed binary reports the released version. Then sync
`project.jsonc` into the jadmadi.net catalog and commit the site data.

## What this gate does not cover

- Cross-platform binaries beyond the CI matrix.
- Third-party security review.
- Marketing claims about popularity or adoption.

Those are decisions, not release blockers, until a release depends on them.
