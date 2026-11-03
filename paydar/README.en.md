# Paydar

**A VPN client for Windows and Android built around a stable connection.** It takes v2ray-style
configs and subscriptions (VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC) and **switches
between them by itself** so the connection stays up. The core is [sing-box](https://github.com/SagerNet/sing-box).

[فارسی](README.md)

## What makes it different

Most clients either stay on one server until you change it, or run a `urltest` every few minutes.
Paydar runs a **guard** that never stops:

| | What it does |
|---|---|
| **Watches the active server** | A real request (HTTP 204) through that server every 5 s. Two misses in a row (the second one second later) count as down. |
| **Fails over at once** | Warm standbys (tested moments ago) are tried first; if none works, **every server races** and the first to answer wins. Usually under 10 s. |
| **Warm standbys** | The best 3 other servers are re-tested every 30 s, so a replacement is always known. |
| **Real scores** | Smoothed latency, jitter and success rate over the last 8 probes; a failing server is benched with a growing cooldown (30 s to 10 min). |
| **Knows when *your* internet is down** | If no server answers, a direct check runs. A dead local network does not mark servers as broken and causes no switching. |
| **No flapping** | Moving to a merely faster server needs a 2x better, stable score, and happens at most once per 10 minutes (each switch resets open connections). |
| **Respects your pick** | A server you chose is left only when it fails, and returned to once it is healthy again. |
| **Network changes** | On Android, a Wi-Fi/mobile switch triggers an immediate check (and a restart with the new DNS if it changed). |
| **No hanging connections** | After a switch, connections on the old server are closed so apps reconnect through the new one instead of stalling on a dead tunnel. |
| **Fast start** | Remembers the last server that worked and starts on it. |
| **Subscriptions** | Auto-update (per `profile-update-interval`, else 12 h), traffic and expiry shown; downloads through the VPN when the subscription host is filtered; a failed update keeps the old list. |

## Features

- **Windows**: TUN mode (whole system, recommended) or system proxy. Tray icon (grey off, amber connecting/no internet, green on). Local SOCKS/HTTP proxy on `127.0.0.1:12334`.
- **Android 8+**: full VPN, Quick Settings tile, notification with the active server and a disconnect button, share links from other apps into Paydar.
- `.ir` domains direct (optional), DNS over HTTPS through the tunnel, LAN always direct.
- Test all servers even while disconnected; remove dead ones with one tap.
- Persian/English UI, light/dark following the system.

## Download

GitHub Actions builds everything on each change:

- Release `paydar-latest`: `paydar-android.apk`, `paydar-setup.exe` (Windows installer), `paydar-*-portable.zip`
- The same files on the `paydar-builds` branch, for networks that filter release downloads

## Build

```bash
go test -tags with_gvisor,with_quic,with_utls,badlinkname,tfogo_checklinkname0 -ldflags=-checklinkname=0 ./...
```

Windows: `scripts\build-windows.ps1 -Version 0.1.0` (Go, Inno Setup 6).
Android: `android/scripts/build-go.sh`, then `./gradlew assembleRelease` (Android SDK and NDK).

## Limits in this version

- Xray-only transports sing-box lacks (XHTTP/SplitHTTP, mKCP, TCP HTTP header) are not supported; adding one says why.
- No WireGuard yet.
- "Iranian sites direct" covers `.ir` domains only, not Iranian IP ranges.

## License

GPL-3.0-or-later (sing-box is GPL). [Vazirmatn](https://github.com/rastikerdar/vazirmatn) font under SIL OFL.
Paydar is an independent project, not affiliated with sing-box or its authors.
