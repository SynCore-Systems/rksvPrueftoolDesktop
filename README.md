# RKSV Prüftool Desktop 🇦🇹

> **🇩🇪 Deutsch zuerst | 🇬🇧 English below**
> **Hinweis:** Dieses Tool führt keine Validierung selbst durch. Es verpackt die offiziellen [BMF-RKSV-Technik](https://github.com/BMF-RKSV-Technik/at-registrierkassen-mustercode) Prüftools in eine Windows-Anwendung – ohne Docker, ohne Java-Installation.

## 🇩🇪 Deutsch

Die Desktop-Variante des [RKSV Web Prüftools](https://github.com/SynCore-Systems/rksvWebPrueftool) für Anwender ohne Server und Docker: eine einzelne `.exe`, Doppelklick, fertig.

### Funktionen
- Prüfen von **DEP-Export** JSON-Dateien
- Prüfen von **QR-Code**-Belegen (Datei oder eingefügt, ein Code pro Zeile)
- Kryptographisches Material als JSON-Datei oder per Formulareingabe
- Eindeutiges Ergebnis (bestanden / fehlgeschlagen) inkl. der konkreten Fehlerursachen aus dem BMF-Prüfbericht
- Wählbare Zeitzone der Belegerstellung (Standard `Europe/Vienna`)
- Verlauf aller Prüfungen; Download von Log und vollständigem Prüfbericht (ZIP)
- Alle Daten bleiben lokal auf dem Rechner

### Voraussetzungen
- Windows 10 oder 11, 64 Bit
- Microsoft Edge WebView2 Runtime – auf Windows 10/11 in der Regel vorinstalliert. Falls sie fehlt, öffnet das Prüftool die Download-Seite.
- Ca. 250 MB freier Speicherplatz
- Für den einmaligen Download des BMF-Prüfmoduls: Internetzugang (oder die Datei manuell bereitstellen, siehe unten)

### Verwendung
1. `RKSV-Prueftool.exe` von den [Releases](https://github.com/SynCore-Systems/rksvPrueftoolDesktop/releases) herunterladen und per Doppelklick starten. Keine Installation und keine Administratorrechte nötig.
2. Beim ersten Start wird die mitgelieferte Java-Laufzeitumgebung entpackt (einige Sekunden).
3. **BMF-Prüfmodul einrichten** (nur beim ersten Start):
   - **„Prüfmodul herunterladen“** lädt `regkassen-verification-1.1.1.zip` vom offiziellen GitHub-Release des BMF.
   - **Ohne Internet / hinter einem Proxy:** die Datei [regkassen-verification-1.1.1.zip](https://github.com/BMF-RKSV-Technik/at-registrierkassen-mustercode/releases/tag/V1.1.1) auf einem anderen Rechner herunterladen und über **„ZIP-Datei auswählen“** einbinden.
   - In beiden Fällen wird die Datei anhand ihrer SHA-256-Prüfsumme geprüft.
4. DEP-Export oder QR-Code und das kryptographische Material auswählen, **Prüfen** klicken.

Die Zeitzone muss der Zeitzone der Belegerstellung entsprechen. Belege, deren Zeitstempel in der Zukunft liegt, schlagen fehl.

### Daten & Deinstallation
Alle Daten liegen unter `%LOCALAPPDATA%\SynCore\RKSV-Prueftool`: Prüfungen (`jobs`), Java-Laufzeit, Prüfmodul und `app.log`. Gespeicherte Prüfungen enthalten das kryptographische Material inkl. AES-Schlüssel. „Alle Daten löschen“ im Verlauf entfernt die Prüfungen.

**Deinstallation:** die `.exe` und den Ordner oben löschen.

### Build
Benötigt nur Docker (das Go-Image wird automatisch verwendet):
```bash
scripts/build.sh 0.1.0
```
| Ausgabe | Inhalt |
|---|---|
| `dist/public/RKSV-Prueftool.exe` | Java-Laufzeit eingebettet, BMF-Prüfmodul wird beim ersten Start geladen. **Wird veröffentlicht.** |
| `dist/internal/RKSV-Prueftool-intern.exe` | Zusätzlich das BMF-Prüfmodul eingebettet, voll offline. **Nur intern** – das BMF-Prüfmodul ist ohne Lizenz veröffentlicht und darf daher nicht weiterverbreitet werden. |
| `dist/linux/rksv-prueftool` | Test-Build ohne Fenster: `rksv-prueftool --serve --data <dir>` (nutzt `java` aus dem PATH oder `$RKSV_JAVA`) |

Die Java-Laufzeit (Eclipse Temurin 8u504) und das BMF-Archiv werden mit festen Versionen und SHA-256-Prüfsummen geladen.

### Architektur
```
RKSV-Prueftool.exe (Go)
 ├─ eingebettet: Temurin 8 JRE, Oberfläche (HTML) [intern: BMF-Prüfmodul]
 ├─ lokaler Dienst auf 127.0.0.1:<zufälliger Port>, geschützt durch ein Sitzungstoken
 ├─ WebView2-Fenster mit der Oberfläche
 └─ java.exe (unsichtbar) -jar regkassen-verification-*.jar …
```

### Lizenz
Apache License 2.0, siehe [LICENSE](LICENSE). Komponenten Dritter: siehe [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

Entwickelt von [SynCore Systems](https://www.SynCore.at).

---

## 🇬🇧 English

The desktop edition of the [RKSV Web Prüftool](https://github.com/SynCore-Systems/rksvWebPrueftool) for users without a server or Docker: a single `.exe`, double-click, done.

### Features
- Validate **DEP export** JSON files
- Validate **QR-code** receipts (file or pasted, one code per line)
- Cryptographic material as a JSON file or via form input
- Clear verdict (pass / fail) including the specific failure causes from the BMF report
- Selectable timezone of receipt creation (default `Europe/Vienna`)
- History of all checks; download of the log and the full verification report (ZIP)
- All data stays on the local machine

### Requirements
- Windows 10 or 11, 64-bit
- Microsoft Edge WebView2 Runtime – usually preinstalled on Windows 10/11. If it's missing, the app opens the download page.
- About 250 MB of free disk space
- For the one-time download of the BMF verification module: internet access (or provide the file manually, see below)

### Usage
1. Download `RKSV-Prueftool.exe` from the [releases](https://github.com/SynCore-Systems/rksvPrueftoolDesktop/releases) and double-click it. No installation and no administrator rights needed.
2. On first start, the bundled Java runtime is unpacked (a few seconds).
3. **Set up the BMF verification module** (first start only):
   - **"Prüfmodul herunterladen"** downloads `regkassen-verification-1.1.1.zip` from the official BMF GitHub release.
   - **Offline / behind a proxy:** download [regkassen-verification-1.1.1.zip](https://github.com/BMF-RKSV-Technik/at-registrierkassen-mustercode/releases/tag/V1.1.1) on another machine and select it via **"ZIP-Datei auswählen"**.
   - In both cases the file is verified against its SHA-256 checksum.
4. Select the DEP export or QR code and the cryptographic material, then click **Prüfen**.

The timezone must match the timezone in which the receipts were created. Receipts whose timestamp lies in the future fail.

### Data & uninstall
All data lives in `%LOCALAPPDATA%\SynCore\RKSV-Prueftool`: checks (`jobs`), Java runtime, verification module and `app.log`. Stored checks contain the cryptographic material, including the AES key. "Alle Daten löschen" in the history removes the checks.

**Uninstall:** delete the `.exe` and the folder above.

### Build
Requires only Docker (the Go image is used automatically):
```bash
scripts/build.sh 0.1.0
```
| Output | Contents |
|---|---|
| `dist/public/RKSV-Prueftool.exe` | Java runtime embedded; the BMF module is downloaded at first start. **Published.** |
| `dist/internal/RKSV-Prueftool-intern.exe` | Also embeds the BMF module, fully offline. **Internal only** – the BMF module is published without a license and must not be redistributed. |
| `dist/linux/rksv-prueftool` | Headless test build: `rksv-prueftool --serve --data <dir>` (uses `java` from the PATH or `$RKSV_JAVA`) |

The Java runtime (Eclipse Temurin 8u504) and the BMF archive are fetched at pinned versions and verified against SHA-256 checksums.

### Architecture
```
RKSV-Prueftool.exe (Go)
 ├─ embedded: Temurin 8 JRE, UI (HTML) [internal: BMF module]
 ├─ local server on 127.0.0.1:<random port>, protected by a session token
 ├─ WebView2 window showing the UI
 └─ java.exe (hidden) -jar regkassen-verification-*.jar …
```

### License
Apache License 2.0, see [LICENSE](LICENSE). Third-party components: see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

Developed by [SynCore Systems](https://www.SynCore.at).
