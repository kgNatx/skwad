# <img src="skwad-icon.svg" width="48" height="48" alt="Skwad icon"> Skwad

A frequency coordinator for FPV drone pilots. When multiple pilots fly together, everyone needs to be on a different video channel to avoid interference. Skwad handles the channel math so pilots can scan a QR code, enter their gear info, and get told which channel to use.

## How It Works

1. One pilot taps **Start Session** and gets a 6-character code
2. Everyone else scans the QR code or enters the code
3. Each pilot picks their video system and gear settings
4. Skwad assigns non-conflicting channels based on everyone's actual signal widths
5. If someone joins and there's no clean slot, they choose: **buddy up** on a shared frequency or **partial rebalance** to shift one pilot and make room

Sessions update live — any join, leave, or channel change is reflected on everyone's screen within seconds.

## Features

- **Smart channel assignment.** The optimizer calculates required spacing for each pilot pair based on their actual signal widths and the session's power ceiling. It's not "stay two channels apart" — it's real RF math.
- **Mixed system support.** Analog, DJI (V1/O3/O4), HDZero, Walksnail, and OpenIPC all in the same session with correct bandwidth-aware spacing.
- **Spotter mode.** Join a session as an observer without taking a frequency slot. Spotters can lead sessions, manage pilots, and switch to a flying video system when they're ready.
- **Session leader controls.** Add/remove pilots, force channel changes, rebalance all assignments, adjust power ceiling, transfer leadership.
- **QR code join.** One tap to share, one scan to join. Works with phone cameras and in-app scanner.
- **Conflict detection.** Real-time danger/warning indicators when pilots are too close, with IMD (intermodulation distortion) scoring.
- **Spectrum visualization.** Live view of the 5.8 GHz band showing each pilot's signal width, conflicts, and IMD products.
- **Fixed channel presets.** Session leader can lock the session to an optimized 2-5 channel set for structured events.
- **14 languages.** Auto-detected from browser, switchable in the footer. Community translations welcome.
- **In-app feedback.** Report bugs, request features, or suggest translation fixes — submissions go straight to GitHub Issues.
- **Usage dashboard.** Anonymous aggregate stats at `/usage` — session counts, video system distribution, and a map of where sessions have been hosted.
- **Installable PWA.** Add to home screen on any device.

## Supported Video Systems

| System | Bandwidth | Notes |
|--------|-----------|-------|
| **Analog** (Race, Fatshark, Boscam E, Low Race) | 20 MHz | Pilot selects which bands their VTX supports |
| **HDZero** | 20 MHz | Race Band frequencies |
| **DJI V1 / Vista** | 20 MHz | Stock (4ch) or FCC unlocked (8ch) |
| **DJI O3** | 20 or 40 MHz | Stock (3ch) or FCC unlocked (7ch at 20MHz, 3ch at 40MHz) |
| **DJI O4 / O4 Pro** | 20, 40, or 60 MHz | Stock/FCC/Race Mode depending on goggles |
| **Walksnail Avatar** | 20 MHz | Standard or Race Mode |
| **OpenIPC** | 20 MHz | Single channel (5825 MHz) |
| **Spotter** | — | Observer, no frequency assignment |

See [frequency-reference.md](frequency-reference.md) for complete channel tables.

## Running It

### Docker

```sh
docker build -t skwad .
docker run -p 8080:8080 -v skwad-data:/data skwad
```

### From Source

Requires Go 1.24+.

```sh
go build -o skwad .
DB_PATH=./skwad.db ./skwad
```

The server starts on port 8080 by default. Set `PORT`, `DB_PATH`, and `STATIC_DIR` environment variables to override.

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/sessions` | Create session |
| `GET` | `/api/sessions/{code}` | Get session state |
| `POST` | `/api/sessions/{code}/join` | Join session |
| `POST` | `/api/sessions/{code}/preview-join` | Preview join (dry-run) |
| `GET` | `/api/sessions/{code}/poll` | Poll for version changes |
| `POST` | `/api/sessions/{code}/rebalance` | Full reoptimize (leader only) |
| `POST` | `/api/sessions/{code}/preview-rebalance` | Preview rebalance (dry-run) |
| `POST` | `/api/sessions/{code}/transfer-leader` | Transfer leadership (leader only) |
| `POST` | `/api/sessions/{code}/add-pilot` | Add pilot (leader only) |
| `POST` | `/api/pilots/{id}/preview-channel?session=CODE` | Preview channel change (dry-run) |
| `PUT` | `/api/pilots/{id}/channel?session=CODE` | Change channel |
| `PUT` | `/api/pilots/{id}/video-system?session=CODE` | Change video system |
| `PUT` | `/api/pilots/{id}/callsign?session=CODE` | Change callsign |
| `DELETE` | `/api/pilots/{id}?session=CODE` | Remove pilot |
| `POST` | `/api/feedback` | Submit feedback |
| `GET` | `/api/usage` | Aggregate usage stats |

Leader-only endpoints check the `X-Pilot-ID` request header against the session's leader. Sessions expire after 12 hours.

## Technical Details

- **Backend:** Go 1.24, stdlib `net/http` router, SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **Frontend:** Vanilla HTML/CSS/JS, no build step, installable as PWA
- **Database:** SQLite with WAL mode
- **Real-time:** Clients poll every 5 seconds (no WebSocket)

For how the optimizer works (graduated escalation, guard bands, buddy groups, fixed channels), see [fpv-optimizer.md](fpv-optimizer.md).

For channel tables and spacing analysis, see [frequency-reference.md](frequency-reference.md) and [docs/channel-separation-guide.md](docs/channel-separation-guide.md).

## Project Structure

```
main.go                  # HTTP server and routing
freq/
  tables.go              # Channel tables, guard bands, bandwidth
  optimizer.go           # Frequency assignment and graduated escalation
api/
  handlers.go            # API endpoint handlers
  feedback.go            # Feedback endpoint (GitHub Issues integration)
db/
  db.go                  # SQLite database layer
static/
  index.html             # Single-page app
  app.js                 # Client-side logic
  style.css              # Styles
  sw.js                  # Service worker (network-first caching)
  i18n.js                # Internationalization module
  locales/               # Translation files (14 languages)
  changelog.html         # What's New page
  freq-guide.html        # Interactive frequency guide
  usage.html             # Usage dashboard
```

## Help Translate Skwad

Skwad supports 14 languages. To request a new language or fix a translation:

1. Open an issue using the [Translation Request](../../issues/new?template=translation-request.yml) template
2. Or submit a PR with a new `static/locales/{lang-code}.json` file based on `static/locales/en.json`

## License

Apache License 2.0. See [LICENSE](LICENSE).
