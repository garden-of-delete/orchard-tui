# orchard-tui

A read-only terminal UI for
[orchard](https://github.com/garden-of-delete/orchard). orchard-tui mirrors
the surface of the [orchard-ui](https://github.com/garden-of-delete/orchard-ui)
Next.js console — list, drill into, and triage workflows, activities,
resources, and stats — all from your terminal.

orchard-tui is **read-only by design**: it never issues a mutating request
against orchard. Use orchard's API (or its web console) to create, activate,
cancel, or delete workflows.

It ships as a single static binary and is designed to **run inside the orchard
pod** against `http://localhost:9000`.

```text
┌──────────────────────────────────────────────────────────┐
│ orchard • http://localhost:9000   ⏸3 ▶12 ✓85 ✗1 [stats] │
├──────────────────────────────────────────────────────────┤
│ Workflows(all)                                           │
│ ID            NAME            STATUS      CREATED        │
│ wf-abc-123    my-pipeline     ▶ running   1h ago         │
│ wf-def-456    nightly-load    ✓ finished  3h ago         │
│ wf-ghi-789    hourly-job      ✗ failed    5h ago         │
│ ...                                                      │
├──────────────────────────────────────────────────────────┤
│ enter·view  /·filter  r·refresh  :·cmd  ?·help  q·quit   │
└──────────────────────────────────────────────────────────┘
```

## Install

### From source
```sh
go install github.com/garden-of-delete/orchard-tui@latest
```

### Build a static Linux binary
```sh
make build-linux
# → bin/orchard-tui-linux-amd64
# → bin/orchard-tui-linux-arm64
```

## Run

```sh
# Defaults to http://localhost:9000 — perfect when running inside the orchard pod.
orchard-tui

# Or point at a remote orchard
ORCHARD_HOST=https://orchard.example.com orchard-tui

# CLI flags override env
orchard-tui --host http://localhost:9001
```

## Deploying into the orchard pod

The binary is statically linked (`CGO_ENABLED=0`) and has no runtime
dependencies. Two common paths:

**Ad-hoc (kubectl cp):**
```sh
make build-linux
kubectl cp bin/orchard-tui-linux-amd64 <pod>:/tmp/orchard-tui
kubectl exec -it <pod> -- /tmp/orchard-tui
```

**Baked into the orchard image:** copy `orchard-tui` into the orchard
container image at build time. With the binary on `$PATH`, an operator
running `kubectl exec -it <pod> -- orchard-tui` gets the TUI immediately.

## Configuration

All configuration is via environment variables.

| Var | Default | Purpose |
|---|---|---|
| `ORCHARD_HOST` | `http://localhost:9000` | orchard base URL |
| `ORCHARD_TUI_API_KEY` | (unset) | Optional `x-api-key` header value |
| `ORCHARD_POLL_FAST` | `2s` | Poll interval for active screens (lists/details with running entities) |
| `ORCHARD_POLL_MEDIUM` | `10s` | Poll interval for header status counts |
| `ORCHARD_POLL_SLOW` | `60s` | Poll interval for the stats screen |
| `ORCHARD_LOG` | (unset) | Optional file path to write logs to |

The `--host` flag overrides `ORCHARD_HOST`. `--print-config` prints the
resolved configuration and exits — handy when debugging in a new pod.

## Keybindings

| Key | Action |
|---|---|
| `j` / `k`, `↓` / `↑` | move selection |
| `g` / `G` | top / bottom |
| `enter` | drill into row |
| `esc` | back / cancel mode |
| `r` | refresh |
| `/` | filter rows (substring; live) |
| `:` | command bar (`:running`, `:wf <id>`, `:stats`, `:q`, …) — `tab` completes |
| `?` | help overlay |
| `0`–`5` | jump to all / pending / running / finished / canceled / failed |
| `s` | jump to stats |
| `tab` | switch Activities ↔ Resources on workflow detail |
| `y` | (in JSON viewer) yank to clipboard |
| `q` / `ctrl+c` | quit |

## Screens

1. **Workflows list** — filterable by status; `/`-search; auto-refresh.
   Fetches up to 200 workflows per request; if the result hits that cap
   the screen shows a truncation hint and you can narrow via `/`, a
   status filter, or `:wf <id>` for direct lookup.
2. **Workflow detail** — header card + tabbed Activities / Resources tables.
3. **Activity detail** — header card + attempts table; `enter` opens the
   attempt's `attemptSpec` in the JSON viewer.
4. **Resource detail** — header card + instances table; `enter` opens the
   instance's `instanceSpec` in the JSON viewer.
5. **Stats** — 30-day stacked daily counts + day-of-week × hour activity
   heatmap.
6. **JSON viewer** — chroma-highlighted, scrollable, `/`-searchable, `y` to
   yank. Shows the relevant AWS console URL for `EmrResource`,
   `Ec2Resource`, and `ShellScriptActivity` types.

## Develop

```sh
make test     # run unit + teatest integration suite
make lint     # gofmt + go vet
make build    # build local binary into ./bin/
make run ARGS="--print-config"
```

The `internal/api` package is fixture-tested against fakes that mirror
orchard's response shapes; `internal/ui` uses
[`teatest`](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest)
to drive the Bubble Tea program with synthetic key events.

### Live integration probe (against a real orchard)

`cmd/integration` is a manual probe that exercises every read endpoint
against a running orchard and prints the decoded responses:

```sh
ORCHARD_HOST=http://localhost:9001 go run ./cmd/integration
```

To stand up orchard locally for that probe, follow its own
[setup instructions](https://github.com/garden-of-delete/orchard).

## License

See [LICENSE.txt](LICENSE.txt).
