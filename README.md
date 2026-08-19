# SoloSet

**One-click Apache Superset, running locally.** SoloSet is a single downloadable
executable that stands up [Apache Superset](https://superset.apache.org/) on your
own machine — no config files, no command line, no setup. Download it, double-click
it, and a few minutes later you're looking at Superset with example dashboards
already loaded.

It's built for solo data folks — consultants, analysts, anyone who wants to spin
up a local data-visualization environment fast without wrestling with Docker,
compose files, and init commands.

---

## What it does

- **Detects Docker** and, if it's missing, offers to **install Docker Desktop for
  you** (via `winget` on Windows or Homebrew on macOS).
- **Launches Superset** in a single self-contained container, with a SQLite
  metadata database kept on a persistent volume.
- **Loads example dashboards** on first run (11 dashboards, 100+ charts, 25 datasets).
- Shows a small **local status page** in your browser with live progress and
  **Start / Stop / Quit** controls.
- Opens Superset for you when it's ready — **`admin` / `admin`** at
  <http://localhost:8088>.

## How it works

SoloSet is a lightweight orchestrator (written in Go) around Docker. When you run
it, it:

1. Serves a status page at `http://127.0.0.1:7654` and opens it in your browser.
2. Checks that Docker is installed and running (installing/starting it if needed).
3. Runs a single `apache/superset` container via an embedded Docker Compose file.
4. On first launch only, runs the one-time setup (database migration, admin user,
   `init`, and example data) — later launches skip straight to ready.
5. Waits for Superset to respond, then hands you an **Open Superset** button.

Everything Superset needs runs inside the container, so SoloSet itself has **no
runtime dependencies** beyond Docker.

## Requirements

- **Docker Desktop** (Windows/macOS). SoloSet can install it for you if it's not
  present. On Windows this uses the WSL2 backend.
- ~2 GB of disk for the Superset image on first run, plus an internet connection
  for the initial download.

## Download & run

Grab the binary for your OS from the [Releases](../../releases) page.

### Windows
Double-click **`SoloSet-windows-amd64.exe`**. Your browser opens to the status
page and setup begins. (Windows may show a SmartScreen prompt for an unsigned
app — choose *More info → Run anyway*.)

### macOS
Because the binary isn't code-signed yet, macOS Gatekeeper will block it on first
open. Either:

- **Right-click** the `SoloSet-macos-*` file → **Open** → **Open**, or
- run once in Terminal to clear the quarantine flag:
  ```sh
  xattr -d com.apple.quarantine ./SoloSet-macos-arm64
  ./SoloSet-macos-arm64
  ```

Use `SoloSet-macos-arm64` on Apple Silicon (M1/M2/M3…) or `SoloSet-macos-amd64`
on Intel Macs.

## Using it

- The status page shows what SoloSet is doing. When it says **Superset is ready**,
  click **Open Superset** and log in with **`admin` / `admin`**.
- **Stop Superset** shuts the container down (your data is kept).
- **Start Superset** brings it back — fast, since setup only runs once.
- **Quit SoloSet** closes the launcher but *leaves Superset running* in Docker, so
  your session stays up. Reopen SoloSet anytime to manage it.

## Data, config & resetting

- Superset's data (dashboards, datasets, users) lives in a Docker volume named
  `soloset_superset_home` and survives restarts.
- SoloSet keeps its compose file and generated secret key in your user config dir
  (`%AppData%\SoloSet` on Windows, `~/Library/Application Support/SoloSet` on macOS).
- To wipe everything and start fresh:
  ```sh
  docker compose -p soloset down -v
  ```

## Building from source

Requires [Go 1.26+](https://go.dev/dl/).

```sh
go build -o SoloSet .     # build for your current OS
go test ./...             # run the tests
```

Cross-compile for another platform:

```sh
GOOS=windows GOARCH=amd64 go build -o SoloSet-windows-amd64.exe .
GOOS=darwin  GOARCH=arm64 go build -o SoloSet-macos-arm64 .
```

Tagging a release (`git tag v0.1.0 && git push --tags`) triggers the GitHub
Actions workflow that builds and publishes binaries for all platforms.

## Troubleshooting

- **"Port 8088 is already in use"** — another program is using Superset's port.
  Close it and click retry.
- **Docker won't start** — open Docker Desktop manually, wait for it to say
  *running*, then click retry in SoloSet.
- **Setup is slow the first time** — that's the image download and example load;
  subsequent launches are quick.

---

*SoloSet is an independent launcher. Apache Superset is a separate project of the
Apache Software Foundation, distributed under the Apache License 2.0.*
