# Rackmon

[![Lizenz: MIT](https://img.shields.io/badge/Lizenz-MIT-blue)](LICENSE)
[![Status: Konzept](https://img.shields.io/badge/Status-Konzept-orange)](docs/konzept.md)

**Deutsch** · [English](README.en.md)

**Ein kleines Display im Rack zeigt, wie es deinen Rechnern gerade geht.** Auf jedem Rechner läuft ein winziger Agent für Linux, macOS oder Windows. Er schickt alle paar Sekunden Prozessorlast, Speicher, Datenträger, Netz und Temperatur an einen Server. Der Server zeigt die Werte live auf einem Mini-Bildschirm an, zum Beispiel einem Streifen-Display mit 1424 x 280 Bildpunkten in einem 10-Zoll-Rack.

## Stand

Rackmon ist ein Konzept. Es gibt noch keinen Code. Der Aufbau steht in [docs/konzept.md](docs/konzept.md).

## Was geplant ist

- **Ein Programm, zwei Modi:** `rackmon agent` sammelt, `rackmon server` sammelt ein und zeigt an. Geschrieben in Go, eine einzige Datei pro Betriebssystem.
- **Agenten senden selbst.** Hinter einer Firewall oder bei wechselnden Adressen muss kein Port geöffnet werden. Jeder Rechner hat ein eigenes Token.
- **Leicht:** unter 15 MB Arbeitsspeicher und kaum Last, auch auf einem Raspberry Pi.
- **Live statt Archiv.** Der Server hält nur den Verlauf der letzten Minuten im Speicher.
- **Eine Anzeige für alle Rechner.** Jeder Rechner bekommt eine Kachel mit Auslastung, Verlauf, Speicher, Datenträger, Netz und Temperatur. Fällt ein Rechner aus, wird seine Kachel rot.
- **Mehrere Bildschirme möglich.** Zu Beginn gibt es einen, die Konfiguration trägt aber schon mehrere.
- **Betriebssysteme:** Linux (auch Raspberry Pi und Proxmox), macOS, Windows.

## Mitmachen

Ideen und Fehler gehören als Issue in dieses Repo. Der Code kommt, sobald das Konzept steht.

## Lizenz

[MIT](LICENSE)
