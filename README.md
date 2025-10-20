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
* Shows a warning when using the default config (127.0.0.1).

---

## Limitations

This application doesn't authenticate with Plex. Therefore it is limited to local control only. Plex doesn't provide any detailed API on local control so features are limited to what has been found/discovered about the local API. 

The main limitation is around starting playback. This seems to only be available through the cloud plex server. So you will need to start the play back through some other controller. IE the mobile app, web app, or my [NFC Controller](https://github.com/spiercey/plexamp-nfc-uart-python).

There is a local playback feature that allows you to start playback on your Plexamp devices using pre-configured playback URLs, similar to NFC tag functionality. See the [PLAYBACK_FEATURE.md](PLAYBACK_FEATURE.md) for more information on how to configure it.

Maybe one day I will update this with Authentication so we can view music and start playback.

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

3. Run the program:

```bash
./plexamp-tui
```

---

## Configuration

### Default Configuration

By default, the program will create a configuration file at:

```
~/.config/plexamp-tui/config.json
```

with the default Plexamp instance:

```json
{
  "instances": ["127.0.0.1"]
}
```

> ⚠️ **Warning:** Using `127.0.0.1` as the default server may not find any running Plexamp instances.
> Update the config file with your server IP(s) to connect properly.

**Editing from the TUI:** You can also add or edit servers directly from within the app:
- Press **`a`** to add a new server
- Press **`e`** to edit the selected server
- Changes are saved automatically to the config file

See the `config.example.json` file for how you can reference your servers. 

### Custom Config Path

You can specify a custom config file with:

```bash
./plexamp-tui --config /path/to/config.json
```

### Config Format

The JSON file should contain an `instances` array with your Plexamp server IPs or hostnames:

```json
{
  "instances": [
    "192.168.1.100",
    "192.168.1.101"
  ]
}
```

---

## Usage

### Navigation

* **↑ / ↓** – Navigate the list of Plexamp instances.
* **Enter** – Select a server.
* **p** – Play / Pause.
* **n** – Next track.
* **b** – Previous track.
* **[ / -** – Decrease volume by 5%.
* **] / +** – Increase volume by 5%.
* **q / Ctrl+C** – Quit the program.

---


## License

This project is licensed under the MIT License.

