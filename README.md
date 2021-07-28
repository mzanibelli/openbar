## OpenBar

A simple status bar for Sway.

See [swaybar-protocol(7)](https://man.archlinux.org/man/swaybar-protocol.7.en).

## Usage

Run `openbar <path-to-configuration-file>`.

Use this command as your Sway `status_command`.

## Configuration

This is an example configuration file.
Only shell commands are supported so far because I don't need anything else, but implementing `openbar.Module` is easy.

```
[
  {
    "command": ["uptime", "-p"],
    "interval": "1m"
  },
  {
    "command": ["date"],
    "interval": "1s"
  }
]
```
