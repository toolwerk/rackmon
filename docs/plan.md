# Rackmon: Implementierungsplan und Stand

Stand: 2026-11-22. Das Konzept steht in [konzept.md](konzept.md). Hier steht, was gebaut ist und was noch offen bleibt.

## Aufbau des Codes

```
cmd/rackmon/            Programmstart: agent, server, top, token, version (Windows-Dienst in agent_run_windows.go)
internal/model/         Messpaket (Sample) und seine Prüfung
internal/agent/         Messen (gopsutil), Senden, Raspberry-Pi-Strom (power_linux.go)
internal/server/        Konfiguration, Speicher mit Zuständen, HTTP, Demo, Textausgabe
internal/web/static/    Dashboard: index.html, style.css, app.js (ein SVG, 1424 x 280)
install/                Dienste und Installationsskripte für Linux, macOS, Windows
configs/                Beispielkonfiguration
```

## Umgesetzt

- [x] Messpaket mit Prüfung (Wertebereiche, Länge, NaN und Unendlich)
- [x] Agent: CPU, Load, RAM, Swap, Datenträger, Netzrate, Uptime, Temperatur, GitHub-Runner-Jobs, Pi-Strom
- [x] Agent sendet im Takt, ohne Warteschlange, meldet Ausfälle nur selten im Protokoll
- [x] Server: Annahme mit Token je Rechner, Zustände ok, stale, offline, Verlauf (5 Minuten), Konfiguration mit Prüfung
- [x] Server: Mehrere Adressen, Sicherheitsköpfe, Größenbegrenzung, sauberes Beenden
- [x] Dashboard: vier Kacheln pro Seite, Blättern mit Animation, Ring, Verläufe, Temperatur mit Grenzlinie, Balken, Netz, Jobs, Offline-Zustand
- [x] Demo-Modus ohne Konfiguration
- [x] Textausgabe: `rackmon top` und `-console` bzw. `console_interval`
- [x] Dienste: systemd (Linux, mit gehärteten Einstellungen), LaunchDaemon (macOS), Windows-Dienst
- [x] Programme für Linux (amd64, arm64, arm), macOS (arm64), Windows (amd64), Release mit Prüfsummen bei einem Tag
- [x] Tests: Modell, Agent (inklusive Senden an einen Testserver), Server (Zustände, Verlauf, Anmeldung, Seiten, Textausgabe, Beispielkonfiguration)
- [x] CI auf Linux, macOS und Windows

## Offen

- [ ] Anzeige an echter Hardware prüfen: Ausgabe-Namen, Zeiten des Displays, Verhalten von HDMI und DisplayPort ([anzeige.md](anzeige.md)). Wartet auf das Kabel.
- [ ] Raspberry Pi und Proxmox-Host anbinden (Zugang und Netz klären), dort den Pi-Strom mit echter Hardware testen.
- [ ] macOS: Temperatur ist bei Apple Silicon je nach System leer, prüfen und gegebenenfalls einen Hilfsdienst mit Rechten ergänzen.
- [ ] Windows: Temperatur über ein Zusatzmodul, falls gewünscht.
- [ ] Ausbau laut [konzept.md](konzept.md), Abschnitt 9: Detailansicht, Touch, Proxmox-API, Meldung bei Ausfall, GitHub-Runner-Status über die API.

## Entscheidungen, die im Lauf gefallen sind

- Agent und Server sind ein Programm, damit es genau eine Datei zu verteilen gibt.
- Das Token liegt immer in einer Datei mit engen Rechten, nie auf der Kommandozeile.
- Der Server setzt die Zeit selbst, die Uhr des Agenten zählt nicht.
- Die Konfiguration lehnt unbekannte Schlüssel ab, damit ein Tippfehler nicht unbemerkt bleibt.
- Das Dashboard wird im Browser aus SVG gezeichnet, es gibt kein Framework und keine Bibliothek zum Nachladen.
