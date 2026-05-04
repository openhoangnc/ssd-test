# SSD Test

A small, single-binary tool to measure the **sustained** write speed of your SSD — past the RAM cache, where the real number lives.

<img width="573" height="336" alt="image" src="https://github.com/user-attachments/assets/53554672-efaf-40d6-b5f5-5f33cbc1c75f" />

## Quick start

Run it (caches the binary for 24h, no $PATH changes):

```sh
curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | sh
```

Pass arguments:

```sh
curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | sh -s -- --size 1G --output /tmp/r.html
```

Install permanently into `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | INSTALL=1 sh
```

Or with Go installed:

```sh
go run github.com/openhoangnc/ssd-test@latest
```

Prebuilt binaries for macOS, Linux, and Windows (x86_64 and arm64) are also published on the [Releases page](https://github.com/openhoangnc/ssd-test/releases/latest).

## What it measures

SSDs ship with a small DRAM or SLC cache that absorbs short bursts of writes at very high speeds. Once that cache fills during sustained writing, throughput drops to the drive's real, NAND-bound rate — often a fraction of the advertised number.

`ssd-test` fills available free space with a continuous stream of random data and reports the throughput live, so you can see exactly when the cache saturates.

## Features

- **Full-screen TUI** with a live sparkline of write speed
- **Hardware report** — disk model, CPU, RAM, OS — auto-detected per platform
- **Self-contained reports** — export an HTML page (with embedded SVG chart) or copy a Markdown summary to the clipboard for instant sharing
- **Auto self-update** — running `ssd-test` with no arguments checks GitHub for a newer release and replaces itself in place (cached for 6 hours; offline-tolerant; bypassed by any flag)
- **Zero third-party dependencies** — pure Go standard library, single binary
- **Cross-platform** — macOS, Linux, Windows; arm64 and x86_64

## Usage

Bare run — self-update, then test current dir:

```sh
ssd-test
```

Test a specific drive with 50% of free space:

```sh
ssd-test --path /mnt/data --size 50%
```

Save a self-contained HTML report:

```sh
ssd-test --output report.html
```

Copy a Markdown summary to the clipboard:

```sh
ssd-test --copy
```

Machine-readable output (for CI):

```sh
ssd-test --json
```

Plain inline output, no TUI:

```sh
ssd-test --simple
```

Skip the self-update check:

```sh
ssd-test --no-update
```

Print version and exit:

```sh
ssd-test --version
```

### Flags

| Flag | Default | Purpose |
|---|---|---|
| `--path` | `.` | Directory to write the test file in |
| `--size` | `auto` | Test size: `auto`, `200M`, `2G`, or `50%` of free space |
| `--output` | — | Write a report (`.html`, `.svg`, or `.md`) |
| `--copy` | `false` | Copy a Markdown summary to the clipboard |
| `--simple` | `false` | Use inline output instead of the TUI |
| `--json` | `false` | Emit a JSON result (implies `--simple`) |
| `--no-update` | `false` | Skip the GitHub release check |
| `--version` | — | Print version and exit |

#### Size argument

| Value | Meaning |
|---|---|
| `auto` (default) | Free space minus a small reserve. The reserve is **1% of the total disk** capped at **1 GiB**, so the drive is never filled completely (which would interfere with normal OS operation). |
| `200M`, `2G`, `500K`, `1T` | Absolute size. Suffixes `K/M/G/T` (or `KiB/MiB/...`) use IEC binary multiples (1024). |
| `50%`, `90%` | Percentage of currently available free space. |

#### Interactive flow

In a TTY (no `--simple` / `--json`), `ssd-test` runs as a full-screen TUI:

1. **Confirm** — shows the device, the bytes about to be written, and the target directory. Press `y` to start or `q` to quit.
2. **Run** — live sparkline of write speed updates each second; metrics pane tracks current / avg / max / min / ETA. Ctrl+C cancels and removes the test file.
3. **Done** — the chart and metrics stay on screen. Footer becomes:

   ```
   [c] copy summary   [h] save HTML report   [q] quit
   ```

   You can pick multiple actions before quitting. HTML reports are saved as `ssd-test-report-<timestamp>.html` in the current directory.

The menu is skipped when stdout/stdin isn't a TTY, with `--simple` / `--json`, or when explicit export flags are given (those run the same actions automatically and exit).

The self-update check only runs on a bare `ssd-test` invocation (no flags) when stdout is a TTY. It can also be disabled with `SSD_TEST_NO_UPDATE=1`.

## Building from source

Requires Go 1.26 or newer.

```sh
git clone https://github.com/openhoangnc/ssd-test.git
```

```sh
cd ssd-test
```

```sh
go build .
```

Cross-compiling is straightforward — the project uses only the Go standard library:

```sh
GOOS=linux   GOARCH=arm64 go build -o ssd-test-linux-arm64 .
```

```sh
GOOS=windows GOARCH=amd64 go build -o ssd-test-windows-amd64.exe .
```

## License

MIT — see [LICENSE](LICENSE).

## Contributing

Issues and pull requests welcome.
