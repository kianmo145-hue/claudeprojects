# Peyvand for Android

Kotlin + Jetpack Compose app with a Go engine (gomobile), in Persian and English (Vazirmatn font).

- **Merge** tab: Wi‑Fi and mobile data at the same time on this phone (local VPN, no server).
- **Booster** tab: share this phone's mobile data with a paired Windows computer on the same Wi‑Fi.
- **Settings**: language, light/dark, even split.

Build: Go, Android SDK + NDK, JDK 17.

```
./scripts/build-go.sh      # golib -> app/libs/peyvand.aar
./gradlew assembleDebug
```

`golib/peyvand` is the API Kotlin calls (`peyvand.Peyvand.*`); `BoostService.kt` runs booster mode,
`MergeVpnService.kt` merge mode. Based on net-merge for Android (MIT).
