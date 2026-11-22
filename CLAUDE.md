# Rackmon - Hinweise für Claude

Leichtes Rack-Monitoring: Agenten (Linux, macOS, Windows) senden Leistungsdaten an einen Server, der sie live auf einem Mini-Display zeigt. Konzept: `docs/konzept.md`. Stand: Konzept, noch kein Code.

## Regeln

- **Öffentliches Repo.** Keine Tokens, keine echten Rechnernamen, IP-Adressen oder internen Domains in Code, Beispielen, Issues und Doku. Beispiele nutzen erfundene Namen wie `rechner-1`.
- Konfigurationsbeispiele als `*.example.yaml`. Echte Konfiguration mit Tokens bleibt lokal (steht in `.gitignore`).
- In README, Doku und Website werden keine Wettbewerber namentlich genannt.
- Sprache: Doku auf Deutsch (README zusätzlich `README.en.md`), Commit-Messages auf Englisch, konventionell und klein.
- Git-Identität: Frank Neumann-Staude, lokal pro Repo gesetzt, keine Firmenadresse.
- Go, ein Programm mit zwei Modi (`agent`, `server`), statisch gelinkt, Cross-Build für linux/amd64, linux/arm64, linux/arm, darwin/arm64, windows/amd64. Sammlung über gopsutil.
- Agenten halten sich unter 15 MB RAM. Jeder neue Messwert muss auf allen drei Betriebssystemen laufen oder leer gemeldet werden.
- Eine Aufgabe, ein Issue, ein Branch `feature/<n>-slug`, PR mit `Closes #<n>`, Squash-Merge.
- `CHANGELOG.md` pro Merge pflegen.
