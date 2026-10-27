# Evergreen

<img src="assets/evergreen.svg" width="96" align="right" alt="Evergreen logo">

Evergreen runs the Roblox Player on Linux. It downloads the macOS build of the
Player, puts it in a [rosemary](https://github.com/microlift/rosemary) prefix
and launches it from there. It registers itself for `roblox://` and
`roblox-player:` links, so the Play button on the website works. Every launch
checks for a new Player version first and installs it if there is one.

Evergreen is a fork of [Everblue](https://github.com/jerr3n/everblue) by
jerr3n, which does the same for Roblox Studio. Most of the code (the Qt window,
the download and extraction, the rosemary calls) comes from there. None of it
would do anything without rosemary by [microlift](https://github.com/microlift),
which runs the macOS app.

## Requirements

- [rosemary](https://github.com/microlift/rosemary) on your `PATH`.
- Go 1.27 or newer.
- SDL3 development files, a C/C++ compiler and `pkg-config`.
- Qt 6 development files for the
  [miqt](https://github.com/mappu/miqt) bindings.

## Building and installing

```sh
make
make test
make install
xdg-mime default evergreen.desktop x-scheme-handler/roblox-player x-scheme-handler/roblox
```

The first build compiles miqt's Qt bindings and takes ten minutes or more.

## Usage

| Command | What it does |
| --- | --- |
| `evergreen [link]` | Update the Player if needed, then launch it, joining the game in `link` if one is given. |
| `evergreen cfg` | Open the Evergreen window. |
| `evergreen install` | Download and install the latest Player, even if it's current. |
| `evergreen run [link]` | Same as `evergreen`, without any window. |

The Player goes to `~/.local/share/rosemary/prefixes/default/root/Applications/Roblox.app`.
If the rosemary prefix doesn't exist yet, Evergreen creates it, which downloads
rosemary's Darwin base (about 2 MB).

If your account is in a Roblox beta program, the Player asks to restart into
the beta's build. Roblox's own updater crashes under rosemary, but it logs
which build it wanted before it does. Evergreen installs that build the next
time you launch, so click the button, relaunch, and you're on the beta.
Evergreen preserves that channel in website join links and restores the beta
build if Roblox's updater replaces it with a public build.

Roblox and rosemary both stop at 60 FPS by default. Evergreen raises that to
240 by setting `DFIntTaskSchedulerTargetFps` in the Player's
`ClientAppSettings.json` (other flags in that file are kept) and by starting
rosemary with `ROSEMARY_SCREEN_REFRESH_RATE=240`. Set that variable yourself to
pick a different rate.

Held keys send one physical press and one release to the Player. Evergreen
filters SDL's repeated key-down events so they cannot reset hold-to-activate
prompts. Separate text-input events are kept, but text fields that rely on
repeated key-downs may no longer repeat characters when a key is held. Set
`EVERGREEN_KEY_REPEAT=1` to restore the original behavior. This filter applies
only to rosemary processes launched by Evergreen.

## License

GPL v3, same as Everblue. See [LICENSE](LICENSE).
