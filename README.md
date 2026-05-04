# SSD Test

A small, single-binary tool to measure the **sustained** write speed of your SSD — past the RAM cache, where the real number lives.

![ssd-test](https://user-images.githubusercontent.com/20717116/207495720-ffb9c971-edf7-4f8a-97bb-e3a87c7e514b.png)

## Quick start

```sh
# Run it. (Caches the binary for 24h, no $PATH changes.)
curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | sh

# Pass arguments
curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | sh -s -- --size 1G --output /tmp/r.html

# Install permanently into ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | INSTALL=1 sh

# Or with Go installed
go run github.com/openhoangnc/ssd-test@latest
```

Prebuilt binaries for macOS, Linux, and Windows (x86_64 and arm64) are also published on the [Releases page](https://github.com/openhoangnc/ssd-test/releases/latest).

## What it measures

SSDs ship with a small DRAM or SLC cache that absorbs short bursts of writes at very high speeds. Once that cache fills during sustained writing, throughput drops to the drive's real, NAND-bound rate — often a fraction of the advertised number.

`ssd-test` fills available free space with a continuous stream of random data and reports the throughput live, so you can see exactly when the cache saturates.

## Features

- **Full-screen TUI** with a live sparkline of write speed
- **Hardware report** — disk model, CPU, RAM, OS — auto-detected per platform
- **Self-contained reports** — export an HTML page (with embedded SVG chart) or a PNG image; copy a Markdown summary to the clipboard for instant sharing
- **Auto self-update** — running `ssd-test` with no arguments checks GitHub for a newer release and replaces itself in place (cached for 6 hours; offline-tolerant; bypassed by any flag)
- **Zero third-party dependencies** — pure Go standard library, single binary
- **Cross-platform** — macOS, Linux, Windows; arm64 and x86_64

## Usage

```sh
ssd-test                              # bare run: self-update, then test current dir
ssd-test --path /mnt/data --size 50%  # test a specific drive with 50% of free space
ssd-test --output report.html         # save a self-contained HTML report
ssd-test --output report.png          # save a PNG of the chart
ssd-test --copy                       # copy a Markdown summary to the clipboard
ssd-test --json                       # machine-readable output (for CI)
ssd-test --simple                     # plain inline output, no TUI
ssd-test --no-update                  # skip the self-update check
ssd-test --version
```

### Flags

| Flag | Default | Purpose |
|---|---|---|
| `--path` | `.` | Directory to write the test file in |
| `--size` | `auto` | Test size: `auto`, `200M`, `2G`, or `50%` of free space |
| `--output` | — | Write a report (`.html`, `.png`, `.svg`, or `.md`) |
| `--copy` | `false` | Copy a Markdown summary to the clipboard |
| `--simple` | `false` | Use inline output instead of the TUI |
| `--json` | `false` | Emit a JSON result (implies `--simple`) |
| `--no-update` | `false` | Skip the GitHub release check |
| `--version` | — | Print version and exit |

The self-update check only runs on a bare `ssd-test` invocation (no flags) when stdout is a TTY. It can also be disabled with `SSD_TEST_NO_UPDATE=1`.

## Building from source

Requires Go 1.26 or newer.

```sh
git clone https://github.com/openhoangnc/ssd-test.git
cd ssd-test
go build .
```

Cross-compiling is straightforward — the project uses only the Go standard library:

```sh
GOOS=linux   GOARCH=arm64 go build -o ssd-test-linux-arm64 .
GOOS=windows GOARCH=amd64 go build -o ssd-test-windows-amd64.exe .
```

## License

MIT — see [LICENSE](LICENSE).

## Contributing

Issues and pull requests welcome.
