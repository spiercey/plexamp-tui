# Playback Selection Feature

## Overview
This feature allows you to start playback on your Plexamp devices using pre-configured playback URLs, similar to NFC tag functionality.

## Configuration

### Config File Location
The playback configuration is stored at:
- `~/.config/plexamp-tui/playback.json` (or `$XDG_CONFIG_HOME/plexamp-tui/playback.json`)

### Config File Format
```json
{
  "items": [
    {
      "name": "Display Name",
      "url": "https://listen.plex.tv/player/playback/playMedia?address=YOUR_SERVER&machineIdentifier=YOUR_MACHINE_ID&key=/library/metadata/12345&type=music"
    }
  ]
}
```

### Getting Playback URLs
You can obtain playback URLs using the same method as the NFC project:
1. Use the Plex web interface or app to find the content you want
2. The URL format follows the pattern: `https://listen.plex.tv/player/playback/playMedia?...`
3. Supported content types:
   - **Tracks/Albums**: URLs containing `/library/metadata/`
   - **Playlists**: URLs containing `/playlists/`
   - **Stations**: URLs containing `/stations/`
   - **Library Sections/Artists**: URLs containing `/sections/`

### How It Works
When you select a playback item:
1. The app takes the `listen.plex.tv` URL from your config
2. Replaces the domain with your selected server's IP (e.g., `192.168.1.100:32500`)
3. Sends the playback request to your local Plexamp instance

This bypasses the need for full authentication while still allowing you to start playback.

## Usage

The left panel can be toggled between two modes:
- **Servers**: Choose which Plexamp instance to control
- **Playlists**: Choose which content to start playing

### Controls

1. **Press `s` or `Tab`** to toggle the left panel between Servers and Playlists
2. **Use ↑/↓** to navigate through items in the current panel
3. **In Servers mode**: Press **Enter** to select a server
4. **In Playlists mode**: Press **Enter or `p`** to start playback on the currently selected server

The panel stays in the mode you selected - it won't automatically switch back after making a selection. This lets you quickly trigger multiple playlists without having to toggle back and forth.

The right panel shows which mode the left panel is currently in ("Left Panel: Servers" or "Left Panel: Playlists").

The app will display "Playback Started" or "Playback Failed" in the status based on the result.

### Debug Mode

To enable debug logging, run the app with the `--debug` flag:
```bash
./plexamp-tui --debug
```

Debug logs will be written to `~/.config/plexamp-tui/playback.log`

## Example Config
See `playback_example.json` for a template configuration file.

## Notes
- Make sure your playback URLs are valid and point to content in your Plex library
- The selected server must be running and accessible
- This feature works the same way as NFC tags - it's a local bypass that doesn't require full Plex authentication
