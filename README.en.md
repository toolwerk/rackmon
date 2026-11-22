# Rackmon

[![License: MIT](https://img.shields.io/badge/License-MIT-blue)](LICENSE)
[![Status: concept](https://img.shields.io/badge/Status-concept-orange)](docs/konzept.md)

[Deutsch](README.md) · **English**

**A small display in your rack shows how your machines are doing.** A tiny agent runs on each machine (Linux, macOS or Windows) and sends CPU load, memory, disk, network and temperature to a server every few seconds. The server shows the values live on a mini screen, for example a 1424 x 280 strip display in a 10-inch rack.

## Status

Rackmon is a concept. There is no code yet. The design is in [docs/konzept.md](docs/konzept.md) (German).

## Planned

- **One program, two modes:** `rackmon agent` collects, `rackmon server` receives and displays. Written in Go, a single file per operating system.
- **Agents push.** No open ports behind a firewall or with changing addresses. Each machine has its own token.
- **Light:** under 15 MB of memory and almost no load, even on a Raspberry Pi.
- **Live, not an archive.** The server keeps only the last few minutes in memory.
- **One display for all machines.** Each machine gets a tile with load, history, memory, disk, network and temperature. A machine that goes silent turns red.
- **Several screens possible.** One at the start, but the configuration already supports more.
- **Operating systems:** Linux (including Raspberry Pi and Proxmox), macOS, Windows.

## License

[MIT](LICENSE)
