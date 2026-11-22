# Anzeige an einem Rechner betreiben

Das Dashboard ist eine Webseite: `http://127.0.0.1:8470/screen/1`. Sie läuft im Kiosk-Modus eines Browsers. Diese Seite beschreibt den Fall, dass der Rechner mit dem Server zwei Bildschirme hat:

- **HDMI: Hauptbildschirm.** Konsole, Anmeldung, Boot-Meldungen und ein IP-KVM.
- **DisplayPort: Monitoring.** Dort läuft nur das Dashboard auf dem Streifen-Display (1424 x 280).

Die Vorgehensweise gilt für Linux mit X-Server. Die Dateien aus dem privaten Einrichtungs-Repo des Autors sind ein Beispiel dafür.

## Wer bekommt den Vorrang

Wenn beide Bildschirme angeschlossen sind, wirkt der DisplayPort oft wie der bevorzugte Ausgang. Das wird auf drei Ebenen festgelegt, jede für sich:

1. **Firmware.** Das Boot-Bild und das BIOS erscheinen dort, wo die Firmware es will. Das lässt sich nur im BIOS ändern, falls es einen "primären Ausgang" gibt. Sonst zeigt das Display beim Start kurz Textmeldungen. Das ist unkritisch.
2. **Linux-Konsole.** Der Kernel legt die Konsole auf die Ausgänge. Die Namen der Ausgänge (`DP-1`, `HDMI-A-1`) liest man mit angeschlossenem Display aus `/sys/class/drm/*/status`. Den Ausgang per Kernel-Parameter zu erzwingen (`video=DP-1:1424x280@60e`) hilft bei einem passiven DisplayPort-auf-HDMI-Kabel nicht: Der Kernel überspringt dann die Adapter-Erkennung, und das Display bekommt ein Signal, das es nicht lesen kann (schwarz, aber beleuchtet). Besser ohne Parameter lassen und das Display nach dem Start einstecken.
3. **X-Server.** HDMI wird `--primary`, der DisplayPort daneben:

   ```bash
   xrandr --output HDMI-A-1 --primary --auto --pos 0x0 \
          --output DP-1 --mode 1424x280 --right-of HDMI-A-1
   ```

   Ein Fenstermanager (Openbox) legt den Browser per Regel auf den zweiten Bildschirm, ohne Rahmen und im Vollbild.

Auf HDMI läuft währenddessen ein Textfenster mit dem Protokoll des Monitorings, damit auch der KVM etwas Sinnvolles sieht. Zurück zur reinen Textkonsole geht es mit Strg+Alt+F1.

## Start und Ende

Ein Timer prüft alle 15 Sekunden, ob am DisplayPort ein Display erkannt wird (`/sys/class/drm/card*-DP-*/status` steht auf `connected`). Erst dann startet der X-Server mit dem Browser, und beim Abziehen des Kabels endet er wieder. Ohne Display am DisplayPort ändert sich nichts: HDMI behält die Textkonsole.

## Hinweise

- Ubuntu liefert Chromium und Firefox nur als Snap. Für den Kiosk den Browser als normales Paket installieren (zum Beispiel Firefox aus dem Mozilla-Repo).
- Bildschirmschoner und Energiesparen aus: `xset s off -dpms s noblank`.
- Meldet das Display seine Zeiten nicht sauber (am DisplayPort-auf-HDMI-Kabel oft ohne Angabe), die Auflösung im X-Server vorgeben: `cvt -r 1424 280 60`, dann `xrandr --newmode` und `--addmode`.
- Die Grafikkarte kann `/dev/dri/card1` heißen, nicht `card0`. Dann findet der X-Server keinen Bildschirm, bis in `xorg.conf.d` `Option "kmsdev" "/dev/dri/card1"` steht.
- Läuft der Browser nicht, genügt zum Prüfen `curl http://127.0.0.1:8470/api/state`. Die Daten kommen vom Server, die Anzeige ist nur eine Webseite.

## Zu prüfen, sobald das Display hängt

1. Heißen die Ausgänge wie angenommen?
2. Bleibt der HDMI-Ausgang nach dem Start des X-Servers lebendig?
3. Stimmen die Zeiten des Displays, ist das Bild scharf und nicht verschoben?
4. Landet der Browser auf dem DisplayPort und nicht auf HDMI (Monitor-Nummer in der Openbox-Regel)?
