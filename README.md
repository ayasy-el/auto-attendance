# Auto Attendance

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-100%25%20Passing-brightgreen)](internal/)

> An automated attendance daemon for university CAS SSO academic portals.

---

## Features

- **Automated Presence:** Detects active class sessions and submits attendance automatically.
- **CAS SSO Authentication:** Authenticates seamlessly with university CAS SSO systems.
- **Telegram Assistant & Alerts:** Real-time presence notifications and two-way control bot (`/status`, `/check`, `/logs`, `/jadwal`, `/tugas`, etc.).
- **SQLite Log Retention:** Persists execution logs in SQLite with daily pruning (`retention_days`) and remote viewing via `/logs [n]`.
- **Academic Queries:** Direct queries for weekly timetable, active assignments, exam schedules, course materials, and class rosters.
- **Smart Scheduling:** Polls dynamically based on your timetable with human-like randomized intervals.
- **Auto-Contract Approval:** Automatically accepts pending course agreements required prior to presence submission.
- **Execution Modes:** Supports continuous background daemon, single-pass check (`-once`), and simulation mode (`-dry-run`).
- **Safe & Resilient:** Graceful shutdown handling and memory-safe response buffers.
- **Docker Ready:** Minimal, secure container setup with non-root execution.

---

## Getting Started

### Prerequisites

- [Go](https://go.dev/dl/) version 1.23 or later (for building from source).
- An active student CAS account.

### Configuration

1. Copy the sample configuration file:
   ```bash
   cp config.yaml.example config.yaml
   ```
2. Copy the sample environment file:
   ```bash
   cp .env.example .env
   ```
3. Set your credentials inside `.env`:
   ```dotenv
   USERNAME=your_username
   PASSWORD=your_password
   ```

### Running the Application

```bash
go mod tidy
go run ./cmd/attendance -config config.yaml
```

---

## Execution Modes & CLI Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-config <path>` | `config.yaml` | Path to the YAML configuration file |
| `-once` | `false` | Run a single attendance check pass and exit immediately (ideal for cron) |
| `-dry-run` | `false` | Simulation mode: checks presence without submitting attendance |

### Examples

```bash
# Continuous daemon (default)
go run ./cmd/attendance -config config.yaml

# Single check pass (e.g. for cron jobs)
go run ./cmd/attendance -config config.yaml -once

# Dry-run simulation (safe testing)
go run ./cmd/attendance -config config.yaml -once -dry-run
```

---

## Telegram Bot Commands

When `telegram.token` and `telegram.chat_id` are configured, the daemon listens for commands and provides an interactive inline keyboard. Unauthorized chat IDs are silently ignored.

| Command | Description |
| :--- | :--- |
| `/status` | Daemon status, operational mode, active courses count, and uptime |
| `/check` | Triggers an immediate attendance scan cycle |
| `/logs [n]` | Displays the last `n` lines of logs from the SQLite store (default: 5, max: 50) |
| `/today` | Lists courses successfully attended today |
| `/whoami` | Displays linked student profile (Name and ID) |
| `/relogin` | Re-authenticates CAS SSO session using in-memory credentials |
| `/pause` | Temporarily pauses automated attendance scanning |
| `/resume` | Resumes automated attendance scanning |
| `/ping` | Connectivity check (replies with `🏓 Pong!`) |
| `/jadwal` | Displays weekly class schedule with room, time, and lecturer |
| `/tugas` | Lists active assignments sorted by deadline with urgency badges (🔴, 🟡, 🟢) |
| `/ujian` | Displays scheduled midterm (UTS) and final (UAS) exam timetables |
| `/materi` | Lists uploaded lecture materials and video recordings |
| `/rekap` | Official attendance rate and total sessions summary |
| `/presensi_kelas` | Displays live attendance roster (attendees and absentees) for active class sessions |
| `/help` | Displays command list and help menu |

---

## Docker Deployment

### Docker Compose

1. Prepare `config.yaml` and `.env`.
2. Start the container in detached mode:
   ```bash
   docker compose up -d
   ```
3. View logs:
   ```bash
   docker compose logs -f auto-attendance
   ```
4. Stop the container:
   ```bash
   docker compose down
   ```

> **Note:** Persistent state and logs (SQLite `log.db`) are managed via Docker's named volume `data`.

---

## Configuration Reference (`config.yaml`)

```yaml
# Portal & CAS Endpoints
host: "https://academic.example.ac.id"
cas_host: "https://login.example.ac.id"

# Credentials (supports environment variable expansion)
username: "${USERNAME}"
password: "${PASSWORD}"

timezone: "Asia/Jakarta"

schedule:
  outside_class_interval:
    min: "10m"
    max: "20m"
  inside_class_interval:
    min: "1m"
    max: "3m"
  active_start: "08:00"
  active_end: "18:00"
  run_immediately: true
  refresh_schedule_interval: "24h"

http:
  timeout: "30s"
  max_response_body: "10MB"
  insecure_skip_verify: true # Ignore expired/untrusted campus TLS certs
  user_agent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"

logging:
  retention_days: 7

# Optional: Telegram bot notifications and interactive commands
telegram:
  token: "${TELEGRAM_TOKEN}"
  chat_id: "${TELEGRAM_CHAT_ID}"
```

| Key | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `host` | string | - | Base URL for the academic portal API |
| `cas_host` | string | - | Base URL for the CAS SSO service |
| `username` / `password` | string | - | Student credentials (supports `${ENV_VAR}`) |
| `timezone` | string | `Asia/Jakarta` | Timezone for schedule calculations |
| `schedule.inside_class_interval` | range | `1m`–`3m` | Polling interval during active class hours |
| `schedule.outside_class_interval` | range | `10m`–`20m` | Polling interval outside class hours |
| `schedule.active_start` / `active_end` | string | `08:00` / `18:00` | Daily active scanning window |
| `schedule.refresh_schedule_interval` | duration | `24h` | Schedule reload interval |
| `http.timeout` | duration | `30s` | HTTP request timeout |
| `http.max_response_body` | string | `10MB` | Maximum response buffer limit |
| `http.insecure_skip_verify` | bool | `false` | Ignore expired or untrusted SSL/TLS certificates |
| `logging.retention_days` | int | `7` | Number of days to retain logs in SQLite (0 = forever) |
| `telegram.token` | string | - | Bot token from [@BotFather](https://t.me/BotFather) |
| `telegram.chat_id` | string | - | Authorized Telegram Chat ID (user or group ID) |

---

## Testing

```bash
go test -v -count=1 ./...
```

---

## Disclaimer

This software is developed independently for educational purposes and personal productivity automation. Users assume full responsibility for their account usage and adherence to institutional policies.
