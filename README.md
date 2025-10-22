# Plexamp TUI

**Plexamp TUI** is a terminal-based controller for [Plexamp](https://plexamp.com) headless instances.
It allows you to select a Plexamp server, view the currently playing track, playback state, progress, and control playback and volume directly from your terminal.

<img width="1278" height="679" alt="image" src="https://github.com/user-attachments/assets/353d9a1b-1a94-43fe-9f6d-a80b2f68c00b" />


---

## Features

* Select and switch between multiple Plexamp instances.
* Displays current track, playback state, progress, and volume.
* Control playback: play/pause, next, previous.
* Control volume: increase or decrease in 5% increments.

---

## Installation

1. Clone this repository:

```bash
git clone https://github.com/<your-username>/plexamp-tui.git
cd plexamp-tui
```

2. Build the Go program:

```bash
go build -o plexamp-tui
```

3. Run the program with auth flag to authenticate with Plex:

```bash
./plexamp-tui --auth
```

4. Follow the instructions to authenticate with Plex.

5. Run the program normally:

```bash
./plexamp-tui
```

Use the Server Selector with 6 to select your server
Use the Playback Selector with 7 to select your playback device. 


Use 1, 2 or 3 to switch between Artist, Albums and Playlists to play. 

---

## Configuration

### Default Configuration

By default, the program will create a configuration file at:

```
~/.config/plexamp-tui/config.json
```

Once in the TUI, you can select your server and playback device using the Server Selector by pressing 6 and Playback selector by pressing 7.


### Custom Config Path

You can specify a custom config file with:

```bash
./plexamp-tui --config /path/to/config.json
```
---

## License

This project is licensed under the MIT License.

