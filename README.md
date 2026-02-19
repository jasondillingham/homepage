# Local Services Dashboard

A lightweight landing page that monitors web services running on your local machine. Built with Go, HTMX, and Bootstrap.

![Dark theme dashboard](https://img.shields.io/badge/theme-dark-0d1117) ![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white) ![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Auto-discovery** — Scans for all TCP services listening on ports 8000–9999 using `lsof`
- **Live refresh** — Auto-polls every 5 seconds via HTMX (toggleable)
- **Stop** — Send SIGTERM to a running process (falls back to SIGKILL after 3s)
- **Restart** — Captures the process command, stops it, and relaunches it
- **Self-protection** — Dashboard prevents you from stopping/restarting itself

## Quick Start

```bash
go build -o homepage .
./homepage
```

Open **http://localhost:8899** in your browser.

## Stack

| Layer    | Technology       |
|----------|-----------------|
| Backend  | Go (net/http)   |
| Frontend | HTMX 2.0        |
| Styling  | Bootstrap 5.3   |
| Theme    | Dark             |

## API Endpoints

| Method | Path       | Description                        |
|--------|------------|------------------------------------|
| GET    | `/`        | Main dashboard page                |
| GET    | `/services`| List running services (HTML/JSON)  |
| POST   | `/stop`    | Stop a process by PID              |
| POST   | `/restart` | Restart a process by PID           |

## Project Structure

```
├── main.go                              # HTTP server + port scanning + process control
├── templates/
│   ├── index.html                       # Main page layout
│   └── partials/
│       └── service_table.html           # HTMX partial for the service table
├── go.mod
└── .gitignore
```
