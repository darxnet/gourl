# Contributing

`gourl` is maintained by one person; issues and pull requests are welcome.
Please open an issue first for anything bigger than a bug fix, so the design is
agreed before the code is written.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org) are required: the
release version is computed from them by [`svu`](https://github.com/caarlos0/svu)
in CI when a commit lands on `main`.

| Prefix | Release |
|---|---|
| `fix:` | patch |
| `feat:` | minor |
| `feat!:` or a `BREAKING CHANGE:` footer | major |
| `chore:`, `docs:`, `test:`, `ci:`, `refactor:`, `perf:` | none |

A `perf:` change that users should know about belongs in a `feat:` commit.

## Before a pull request

    go mod tidy
    go generate ./...                      # encoding_table.go must be up to date
    go vet ./...
    go test -race -count=1 ./...
    golangci-lint run ./...                # .golangci.yml, `default: all`

CI runs the same checks on the stable and oldstable Go releases, on Linux (amd64) and macOS (arm64),
plus `govulncheck`, a coverage gate and the comparison benchmarks in
[benchmarks](benchmarks).

## Changes to parsing or escaping

The rules are those of `net/url`; a change must keep them. `std_test.go`
checks every input against `net/url` and fuzzes the parser, the query parser
and the escaping against it (Go 1.26 or later):

    go test -run=NONE -fuzz='^FuzzParse$' -fuzztime=60s .
    go test -run=NONE -fuzz='^FuzzParseQuery$' -fuzztime=60s .
    go test -run=NONE -fuzz='^FuzzEscape$' -fuzztime=60s .

A deliberate difference from `net/url` is documented in the README and in the
package documentation, and excluded from the differential test with a comment
saying why.

## Changes on a hot path

Include `benchstat` output in the pull request description:

    go test -run=NONE -bench=. -benchmem -count=6 ./... > old.txt   # on main
    go test -run=NONE -bench=. -benchmem -count=6 ./... > new.txt   # on your branch
    benchstat old.txt new.txt

A change that makes a benchmark slower needs a reason in the commit message.
`testing.AllocsPerRun` tests keep every `Append*` operation and `ParseInto`
at zero allocations; a change that adds one needs a very good reason.

## Ground rules

- Every behaviour change comes with a test; coverage is gated in CI.
- No new dependencies: the module deliberately uses only the standard library.
  Comparison benchmarks live in the separate `benchmarks` module.
- Fields and results are views into the caller's data, never copies; decoding
  writes into a buffer the caller provides.
