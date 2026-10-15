# Third-party notices

RKSV Prüftool Desktop is licensed under the [Apache License 2.0](LICENSE). The executable contains or uses the following third-party components.

## Embedded in the executable

| Component | Version | License | Source |
|---|---|---|---|
| Eclipse Temurin JRE (OpenJDK) | 8u504-b01 | GPL v2 with Classpath Exception | <https://adoptium.net>, source: <https://github.com/adoptium/jdk8u> |
| go-webview2 | 2026-02-05 (`5659883`) | MIT (© John Chadwick, Serge Zaitsev) | <https://github.com/jchv/go-webview2> |
| WebView2Loader.dll (Microsoft WebView2 SDK, via go-webview2) | – | BSD-3-Clause style (© Microsoft Corporation) | <https://www.nuget.org/packages/Microsoft.Web.WebView2> |
| go-winloader | 2025-04-06 | ISC (© John Chadwick) | <https://github.com/jchv/go-winloader> |
| golang.org/x/sys | v0.49.0 | BSD-3-Clause (© The Go Authors) | <https://go.googlesource.com/sys> |
| Go standard library / runtime | 1.27.2 | BSD-3-Clause (© The Go Authors) | <https://go.dev> |

The Temurin runtime is unpacked unchanged to `%LOCALAPPDATA%\SynCore\RKSV-Prueftool\runtime\`. Its license texts, including the Classpath Exception and the third-party notices of OpenJDK, are in the runtime's `legal` folder and `NOTICE` file.

At build time, go-winres (MIT, <https://github.com/tc-hib/go-winres>) generates the Windows resources (icon, version information, manifest). It is not part of the executable.

## BMF verification engine (regkassen-verification 1.1.1)

The checks themselves are performed by the official verification tools of the Austrian Federal Ministry of Finance (BMF), published at <https://github.com/BMF-RKSV-Technik/at-registrierkassen-mustercode/releases/tag/V1.1.1>.

- **Public build (`RKSV-Prueftool.exe`):** does **not** contain these tools. On first start, the user downloads `regkassen-verification-1.1.1.zip` from the official release, or selects a copy they downloaded themselves. The app verifies the file against its SHA-256 checksum (`2c1d65b7b0024262e36ebbfba296c47773c68c7000a6f33c36f80b1215e414b6`) before using it unchanged.
- **Offline edition (`rksvPrueftoolDesktop-Offline.exe`, `-tags bundled`):** contains that zip unchanged. The BMF publishes the verification core (`regkassen-verification-core`, `regkassen-verification-common`, `regkassen-testdb` 1.0.42) and the command-line tools without a license. The offline edition is therefore only published once the BMF permits redistribution.

The zip also contains these libraries:

| Library | License |
|---|---|
| registrierkassen-core 0.12, regkassen-common 0.14 (A-SIT Plus GmbH) | Apache 2.0 |
| Jackson 2.7, Guava 15, Commons CLI/Codec/IO/Logging/Math3, PDFBox/FontBox/JempBox 1.8.10, jose4j 0.4.4, Nimbus JOSE+JWT 4.2, json-smart 1.3.1, JCommander 1.27, TestNG 6.8.21, Joda-Time 2.4, ZXing core 2.0 | Apache 2.0 |
| Bouncy Castle 1.52 (bcprov, bcpkix) | MIT-style Bouncy Castle License |
| SLF4J API 1.7.6 | MIT |
| JUnit 4.12 | EPL 1.0 |
| Hamcrest 1.3 | BSD-3-Clause |
| Woodstox StAX2 API 3.1.4 | BSD-2-Clause |
| Javassist 3.19 | MPL 1.1 / LGPL 2.1 / Apache 2.0 |
| BeanShell 2.0b4 | LGPL / SPL |
| FindBugs annotations 2.0.1 | LGPL |
| JCIP annotations 1.0 | CC BY 2.5 |
| Reflections 0.9.10 | WTFPL |
