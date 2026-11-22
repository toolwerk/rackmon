# Rackmon: Konzept

Stand: 2026-11-22. Das Konzept ist umgesetzt, was gebaut ist und was offen bleibt, steht in [plan.md](plan.md).

Ein Live-Display im Rack zeigt Leistungsdaten von mehreren Rechnern. Ein Server nimmt die Werte an und zeigt sie an, auf jedem Rechner sendet ein kleiner Agent.

Als Referenz dient ein 6,91-Zoll-Streifen-Display mit 1424 x 280 Bildpunkten im 10-Zoll-Rack. Das Konzept trägt auch andere Größen.

## 1. Ziele und Abgrenzung

- Live-Ansicht, kein Langzeitarchiv. Verlauf nur kurz (Verlaufslinien der letzten Minuten).
- Ein Bildschirm am Anfang, die Konfiguration trägt aber schon mehrere.
- Touch ist zunächst nicht Teil des Umfangs. Das Layout darf ihn später nicht verbauen.
- Eine Server-Komponente, je ein Agent für Linux, macOS und Windows.
- Agenten sind leicht: wenige MB Arbeitsspeicher, praktisch keine Last.

## 2. Beispielaufbau

Acht Rechner auf einem Bildschirm:

| Kachel | Rolle | Betriebssystem | Build-Ziel |
|---|---|---|---|
| mac-1 | Mac mini | macOS (arm64) | darwin/arm64 |
| win-1 | Mini-PC | Windows | windows/amd64 |
| lin-1 | Mini-PC, zugleich Server und Anzeige | Linux | linux/amd64 |
| pve-1 | Virtualisierungshost | Debian (Proxmox) | linux/amd64 |
| pi-1 bis pi-4 | vier Raspberry Pi | Raspberry Pi OS | linux/arm64 oder linux/arm |

## 3. Architektur

```
 mac-1   win-1   pve-1   pi-1..4            lin-1
 [Agent] [Agent] [Agent] [Agent]            [Agent] ----+
    \       \       |      /                            |
     +-------+------+-----+---- HTTP POST ----> [rackmon server]
        (alle 2 s, Bearer-Token, besser in einem VPN)         |
                                              Ringpuffer im RAM, SSE-Stream
                                                              |
                                              Browser im Kiosk-Modus
                                                              |
                                                   Video-Ausgang -> Display
```

Entscheidungen:

- **Agenten senden (Push), der Server fragt nicht ab.** Rechner hinter Firewalls oder mit wechselnden Adressen brauchen keine offenen Ports. Der Server kennt nur ein Token pro Rechner.
- **Ein Programm, zwei Modi:** `rackmon agent` und `rackmon server`. Geschrieben in Go, statisch gelinkt, Cross-Build für alle Ziele. Die Sammlung läuft über die Bibliothek gopsutil, damit Linux, macOS und Windows dieselben Felder liefern.
- **Transport in einem privaten Netz** (VPN), damit der Verkehr verschlüsselt ist und die Namen stabil bleiben. Wer keins hat, bindet den Server an die Adresse im LAN und lässt das Token den Rest tun.
- **Der Server speichert im Arbeitsspeicher** (Ringpuffer, zum Beispiel eine Stunde bei 2 s Takt, einige Tausend Werte pro Rechner). Ein Schnappschuss auf der Platte hilft bei Neustart. Keine Datenbank nötig.
- **Das Dashboard ist eine statische Seite**, vom Server ausgeliefert. Der Browser bekommt Updates per Server-Sent Events. Kein Framework, Verlaufslinien und Balken als SVG.

## 4. Was die Agenten erfassen

Takt: alle 2 Sekunden ein JSON-Paket, wenige hundert Byte.

| Wert | Linux | macOS | Windows |
|---|---|---|---|
| CPU gesamt in % | ja | ja | ja |
| Load-Average | ja | ja | nein (Platzhalter) |
| RAM belegt in % und absolut | ja | ja | ja |
| Swap bzw. Auslagerungsdatei | ja | ja | ja |
| Datenträger belegt in % (Hauptlaufwerk) | ja | ja | ja |
| Netz Rx/Tx pro Sekunde | ja | ja | ja |
| Uptime | ja | ja | ja |
| CPU-Temperatur | ja (Thermal Zones) | nur mit Root-Helfer, sonst leer | nur mit Zusatzmodul, sonst leer |
| Raspberry Pi: Unterspannung, Drosselung | ja (`vcgencmd get_throttled`) | nein | nein |
| laufende CI-Jobs (optional) | ja | ja | ja |

Fehlende Werte werden leer gemeldet, die Kachel zeigt dann einen Strich. Temperatur ist überall "wenn verfügbar".

Beispielpaket:

```json
{ "host": "lin-1", "os": "linux", "ts": 1763820000,
  "cpu": 12.4, "load1": 0.41, "mem": 38.2, "mem_used_gb": 6.1, "swap": 0.0,
  "disk": 9.8, "net_rx": 18400, "net_tx": 5200, "uptime": 86400,
  "temp": 51.0, "ci_jobs": 1, "throttled": null }
```

Ressourcenbudget für den Agenten: unter 15 MB RAM, unter 1 % CPU auf einem Raspberry Pi.

## 5. Server

- Nimmt Pakete unter `POST /ingest` an und prüft das Token des Rechners.
- Hält je Rechner den letzten Stand, den Verlauf (Ringpuffer) und `last_seen`.
- Zustände: **ok** (Paket jünger als 6 s), **stale** (6 bis 30 s, Kachel gedimmt), **offline** (älter als 30 s, Kachel rot mit "seit hh:mm").
- Liefert `/screen/1` (Dashboard), `/events` (SSE) und `/api/state` (JSON) aus.
- Gibt auf Wunsch alle empfangenen Werte als reinen Text aus, ohne Graphen: `rackmon top` zeigt sie live, `console_interval` schreibt sie ins Protokoll.
- Konfiguration in einer Datei: Rechner mit Namen, Gruppe und Token, Grenzwerte, Bildschirme mit Kachelzuordnung. Ein Beispiel liegt als `rackmon.example.yaml` im Repo, echte Tokens bleiben lokal.
- Der Server bindet nur an die Adresse im privaten Netz oder LAN.

## 6. Bildschirm-Layout (1424 x 280)

Vier Spalten, jede Kachel 340 x 246 px. Es sind aber mehr als vier Rechner: Die Anzeige **blättert seitenweise**. Seite 1 zeigt die ersten vier Rechner, nach einigen Sekunden gleitet Seite 2 mit den nächsten vier herein. Punkte unten in der Mitte zeigen die aktuelle Seite. Der Takt ist einstellbar (Vorgabe 8 Sekunden). Mit mehr Rechnern kommen weitere Seiten dazu.

Weil eine Kachel die ganze Höhe nutzt, gibt es Platz für echte Graphen statt nur Zahlen:

```
+----------------------------------------+
| * ci-mac-01              Mac mini      |   Name, Statuspunkt, Rolle
|  (ring 34%)   Prozessor, 5 Minuten     |   Ring + Verlauf als Fläche
|               ~~~~~~~~~~~~~~~~~~~~~~   |
|  52°          Grenze 75°               |   Temperatur groß + Verlauf
|  Temperatur   ~~~~~~~~~~~~~~~~~~~~~~   |   mit gestrichelter Grenzlinie
|  RAM   ######----   61 %  9,8 GB       |
|  Disk  ####------   38 %               |
|  v 1,2 MB/s  ^ 40 KB/s  [3 Jobs]  12 Tage |
+----------------------------------------+
```

- **Kopf:** Statuspunkt, Name, Rolle. Offline: roter Punkt, Kachel gedimmt, "offline seit hh:mm".
- **Prozessor:** Ring mit Prozentzahl und daneben ein Flächenverlauf der letzten fünf Minuten mit Endpunkt.
- **Temperatur:** große Zahl, daneben ein Verlauf mit gestrichelter Grenzlinie. Die Farbe folgt dem Wert.
- **RAM und Datenträger:** zwei schmale Balken mit Prozent und, wo es passt, absoluten Werten.
- **Fuß:** Netz Rx/Tx, Zahl der laufenden CI-Jobs als Marke, Uptime.
- **Farben:** blau für den Prozessor, Temperatur in Orange. Ab 70 % gelb, ab 90 % rot (Prozessor, Speicher, Datenträger). Temperaturgrenzen sind einstellbar.
- Raspberry Pi mit Unterspannung: Blitz-Symbol an der Temperatur.
- Dunkles Thema, flache Flächen, keine Verläufe oder Schatten. Das Display blendet im Rack nicht und bleibt auch bei kleinen Schriften lesbar.
- Fußstreifen: Uhrzeit links, "n von m online" rechts, Seitenpunkte in der Mitte.

Die fertige Anzeige zeigen `docs/images/seite-1.png` und `docs/images/seite-2.png`.

## 7. Anzeige am Server-Rechner

Der Rechner, der den Server betreibt, treibt auch das Display. Er hat zwei Video-Ausgänge:

- **Hauptbildschirm: HDMI.** Konsole, Anmeldung und Notfallzugriff (zum Beispiel über einen IP-KVM) erscheinen dort.
- **Monitoring: DisplayPort.** Dort läuft nur das Dashboard.

Beobachtung aus der Praxis: Sind beide Monitore angeschlossen, kann der DisplayPort wie der bevorzugte Ausgang wirken. Das muss ausdrücklich umgedreht werden, sonst landet die Konsole auf dem kleinen Display. Drei Ebenen, jede wird einzeln geprüft:

1. **Firmware (BIOS/UEFI):** Boot-Bild und BIOS erscheinen auf dem Ausgang, den die Firmware wählt. Das lässt sich nur im BIOS ändern, falls es eine Einstellung für den primären Ausgang gibt. Lässt es sich nicht ändern, zeigt das Display beim Booten kurz Textmeldungen. Das ist unkritisch.
2. **Linux-Konsole:** Die Konsole wird auf HDMI festgelegt. Per Kernel-Parameter wird der DisplayPort aus der Konsole herausgehalten oder auf seine feste Auflösung gesetzt (`video=DP-1:1424x280@60`). Die Namen der Ausgänge werden vorher am Gerät gelesen.
3. **X-Server für den Kiosk:** HDMI wird als **primär** gesetzt (`xrandr --output HDMI-A-1 --primary`), der DisplayPort als zusätzlicher Ausgang daneben (`--output DP-1 --mode 1424x280 --right-of HDMI-A-1`). Das Dashboard-Fenster liegt genau auf dem Bereich des DisplayPorts. Auf dem HDMI-Ausgang läuft ein einfaches Terminal, damit ein angeschlossener KVM weiter ein sinnvolles Bild bekommt.

Weiter gilt:

- Der Browser startet im Kiosk-Modus mit `http://127.0.0.1:8470/screen/1` auf dem DisplayPort, als eigener Benutzer per systemd, beim Start ohne Anmeldung.
- Auflösung fest vorgeben, falls das Display seine Zeiten nicht sauber meldet.
- Bildschirmschoner und Energiesparen sind aus. Ein Wächter startet den Browser neu, wenn er abstürzt.
- Ubuntu liefert Chromium und Firefox nur als Snap. Für den Kiosk besser den Browser als normales Paket installieren.
- Zu prüfen, sobald das Display angeschlossen ist: Namen der Ausgänge, ob der HDMI-Ausgang nach dem Start des X-Servers weiter ein Bild liefert, ob die Zeiten des Displays stimmen.

## 8. Installation der Agenten

| Ziel | Dienst | Skript |
|---|---|---|
| Linux, Proxmox, Raspberry Pi | systemd-Dienst, eigener Benutzer ohne Anmeldung | `install/agent-linux.sh` |
| macOS | LaunchDaemon | `install/agent-mac.sh` |
| Windows | Windows-Dienst | `install/agent-windows.ps1` |

- Jedes Skript legt Programm, Konfiguration (Server-Adresse, Token) und Dienst an und ist wiederholbar.
- Der Server zeigt je Rechner die Agent-Version, ein Update geschieht per Skript.

## 9. Ausbau

- Weitere Bildschirme (Detailansicht je Rechner, Runner-Auslastung), Touch zum Umschalten und Anhalten des Blätterns.
- Proxmox-Daten über die API: Zahl und Zustand der virtuellen Maschinen.
- Meldung bei Ausfall (zum Beispiel per Push-Nachricht), wenn ein Rechner länger offline ist.
- Runner-Status über die GitHub-API: beschäftigt, frei, offline, laufender Job.
- Längerer Verlauf in einer kleinen Datenbank.
