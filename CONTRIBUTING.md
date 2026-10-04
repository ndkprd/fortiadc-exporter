# Contributing

## Setup

Requires Go 1.26+.

```sh
git clone git@gitlab.com:endekasoft/fortiadc-exporter.git
cd fortiadc-exporter
go build ./...
```

## Development

```sh
go build ./...      # build
go vet ./...        # static checks
gofmt -l .          # list unformatted files (should be empty)
go test ./...       # run tests
```

A local run against a real appliance needs a `config.yml`: copy `config.yml.example` and fill in your FortiADC target(s) and a read-only REST API administrator key. Never commit `config.yml` or real fixtures; `scripts/capture-fixtures.sh` masks names, IPs and serials, review its output before committing.

## Making changes

- Keep changes scoped to what the issue asks for, no drive-by refactors.
- Add or update tests for any behavior change (`*_test.go` next to the code).
- Update `docs/metrics.md` when adding or changing a metric, and check the scrape output with `promtool check metrics`.

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, ...).

## Reporting bugs / requesting features

Open an issue on the [GitLab issue tracker](https://gitlab.com/endekasoft/fortiadc-exporter/-/issues) before writing any code.

## Submitting changes

1. Start from an issue, and wait until it is assigned to you.
2. Branch off `main`.
3. Make sure `go build ./...`, `go vet ./...` and `go test ./...` pass.
4. Open a merge request against `main`, linking the issue it resolves.

## License

By contributing, you agree your contributions are licensed under this project's [MIT License](LICENSE).
