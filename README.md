# cmdk

Keyboard-driven tmux launcher: a fuzzy-filterable popup TUI for switching windows, opening directories, and running shell commands.

## Installation

### Homebrew (macOS)

```sh
brew tap jmcampanini/cmdk https://github.com/jmcampanini/cmdk
brew install --HEAD jmcampanini/cmdk/cmdk
```

To upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD cmdk
```

### From source

```sh
make build
# binary at ./build/cmdk — copy or symlink onto your PATH
```

## Quickstart

Bind a key in `~/.tmux.conf` to launch cmdk in a tmux popup:

```tmux
bind-key Space display-popup -E "cmdk --pane-id=#{pane_id}"
```

Reload tmux (`tmux source-file ~/.tmux.conf`), then press `prefix + Space` inside any tmux session to open the launcher.

## Reference

For the full command reference, run `cmdk --help`. See also:

- `cmdk help exit-codes` — exit codes and error categories
- `cmdk config --provenance` — effective configuration and source provenance
- `cmdk logs --help` — where logs go and how to inspect them
- `cmdk session resolve <path>` — inspect the planned session for a path
- `cmdk session window <path> --new` — create a new window in the cmdk-managed tmux session for a path
- `cmdk docs` — configuration reference
- `cmdk icons` — supported icon aliases
