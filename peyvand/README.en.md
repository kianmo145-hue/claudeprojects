# Peyvand (پیوند)

**Use all your internet connections at once**: Wi‑Fi, Ethernet, a phone tether, and even **a phone's
mobile data over the same Wi‑Fi**. Windows and Android, in Persian and English.

[فارسی](README.md) · [How it works (Persian)](docs/HOW-IT-WORKS.fa.md) · [Booster guide](docs/BOOSTER.md)

## Three modes

| Mode | Where | What it does |
|---|---|---|
| **1. Merge on Windows** | Peyvand tray icon | Uses Wi‑Fi + Ethernet + USB tether / hotspot together |
| **2. Merge on the phone** | Android app, *Merge* tab | Uses the phone's own Wi‑Fi and mobile data together |
| **3. Booster** | Android *Booster* tab + Windows tray | The phone shares its mobile data with the computer **over the Wi‑Fi they share** (no cable, no hotspot); the computer merges it with its own Wi‑Fi |

Peyvand gives each new connection to one of the links, faster links get more, links without internet get
none. Apps that open many connections (download managers, torrents, updates, browsers) get the sum of the
links; a single connection stays on one link.

## Get it

- **Windows**: download `peyvand-windows` from the latest successful *Peyvand* run under **Actions**, run
  `peyvand-setup-*.exe`. The tray icon is grey (paused), amber (one connection) or green (merging), with a
  phone badge when a booster phone is in use.
- **Android 8+**: download `peyvand-android` from the same run and install `peyvand-android.apk`.

### Pair a phone (once)

1. Phone and computer on the same Wi‑Fi; Booster on in the phone app.
2. On Windows: tray icon › **Phones** › your phone › **Pair with this phone…**
3. Both screens show a six-digit code. If they match, tap **Approve** on the phone and click **OK** on the computer.

## Layout

```
peyvand/
  boost/     booster protocol (discovery, code pairing, tunnels), shared, Go standard library only
  desktop/   Windows app (Go): merge engine, Wintun, tray menu, installer
  android/   Android app: Kotlin + Jetpack Compose, Go engine via gomobile
  docs/      how it works, guides
```

## Limits

- The booster carries **TCP** only in this version; UDP (QUIC, calls) stays on the computer's own links.
- Android allows one VPN at a time; merge and booster are one or the other.
- Mobile data is used while merge or booster is on.

## License

MIT. Based on [net-merge](https://github.com/net-merge/net-merge) by Rohit Mahto and its
[Android port](https://github.com/net-merge/android) (both MIT). App font: [Vazirmatn](https://github.com/rastikerdar/vazirmatn) (SIL OFL).
