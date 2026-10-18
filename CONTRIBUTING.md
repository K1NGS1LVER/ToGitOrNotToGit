# Contributing to ToGitOrNotToGit (`tocommit`)

Thanks for helping turn commit messages into theatre. This guide covers local setup,
the checks CI runs, and how releases are produced.

## Prerequisites

- **Go 1.22+** (see `go.mod`)
- `git`
- Optional: [`golangci-lint`](https://golangci-lint.run/) for local linting
- Optional: [`goreleaser`](https://goreleaser.com/) for release dry runs

## Local development

```sh
make build      # builds ./tocommit with version injected from git describe
make test       # go test -race -cover ./...
make vet        # go vet ./...
make fmt        # gofmt -w .
make lint       # golangci-lint run
make cross      # cross-compile the 5 release targets into dist/
```

## Manual hook testing

`tocommit` installs a `prepare-commit-msg` hook. To exercise it against a scratch repo:

```sh
make build
mkdir /tmp/tocommit-sandbox && cd /tmp/tocommit-sandbox
git init -q && git config user.email t@e.st && git config user.name Test
/path/to/tocommit install
echo "package main" > main.go && git add main.go
TOCOMMIT_DEBUG=1 git commit    # watch stderr for provider/persona/fallback reasons
```

`TOCOMMIT_DEBUG=1` prints the chosen provider/model/persona and the reason for any
fallback (missing key, timeout, HTTP status, invalid message, config error).

## Checks CI runs

Pull requests must pass:

1. `gofmt -l .` produces no output
2. `go vet ./...`
3. `go test -race -cover ./...`
4. `golangci-lint run` (config in `.golangci.yml`)

Match the existing style: table-friendly tests, small focused functions, and no
behavior that blocks a user's commit (the hook must always fall back gracefully).

## Releasing

Releases are tag-driven and automated with GoReleaser:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

The `Release` workflow builds `darwin`/`linux`/`windows` × `amd64`/`arm64`, injects the
version via `-ldflags`, publishes archives plus `checksums.txt`, and creates the GitHub
Release. To preview locally before tagging:

```sh
goreleaser release --snapshot --clean
```
