## OpenBar

A simple status bar for Sway.

See [swaybar-protocol(7)](https://man.archlinux.org/man/swaybar-protocol.7.en).

## Usage

Run `openbar <path-to-configuration-file>`.

Use this command as your Sway `status_command`.

You can reload each module manually by emitting a signal equal to `SIGRTMIN+index`, where `index` is the position of the module in the order of declaration.
If you have so many modules that `SIGRTMAX` is reached, the automatically assigned signal cycles back to `SIGRTMIN` for the next module.

Additionally, all modules will reload upon receiving `SIGUSR1`.

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
