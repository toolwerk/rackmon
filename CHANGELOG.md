# Changelog

Alle Änderungen stehen hier, nach [Keep a Changelog](https://keepachangelog.com/de/1.1.0/). Versionen folgen [SemVer](https://semver.org/lang/de/).

## [Unreleased]

## [0.2.0] - 2026-11-22

### Added

- Agent für Linux, macOS und Windows (Prozessor, Speicher, Swap, Datenträger, Netz, Uptime, Temperatur, GitHub-Runner-Jobs, Raspberry-Pi-Strom).
- Server mit Token je Rechner, Zuständen ok, stale und offline, kurzem Verlauf, Prüfung der Pakete und mehreren Adressen.
- Dashboard für 1424 x 280 Bildpunkte mit Blättern, Ring, Verläufen, Temperatur mit Grenzlinie und Offline-Zustand.
- Demo-Modus `rackmon server -demo`.
- Textausgabe ohne Graphen: `rackmon top`, `-console` und `console_interval`.
- Dienste und Installationsskripte für systemd, LaunchDaemon und den Windows-Dienst.
- Konzept, Implementierungsplan und Anleitung für die Anzeige in `docs/`.
- CI auf Linux, macOS und Windows, Release mit Prüfsummen bei einem Tag.
