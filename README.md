# cmdk

Keyboard-driven tmux launcher: a fuzzy-filterable popup TUI for switching windows, opening directories, and running shell commands.

## Requirements and support

- tmux 3.2 or newer is required for the interactive launcher and tmux-backed commands.
- Linux and macOS are supported. Linux is verified in required CI; other Unix-like systems are unverified, and Windows is unsupported.
- [zoxide](https://github.com/ajeetdsouza/zoxide) is optional and enables zoxide-backed directory entries.
- A [Nerd Font](https://www.nerdfonts.com/) is optional for enhanced icons.

Install or upgrade tmux with your platform package manager, then verify the version with `tmux -V`:

```sh
brew install tmux                 # macOS
sudo apt-get install tmux         # Debian or Ubuntu
```

Use `brew upgrade tmux` on macOS or your Linux package manager's upgrade command when an installed version is older than 3.2.

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

### From source (Linux or macOS)

With Git, Go, Make, and tmux 3.2 or newer installed:

```sh
git clone https://github.com/jmcampanini/cmdk.git
cd cmdk
make build
# Copy or symlink ./build/cmdk onto your PATH.
```

To upgrade, run `git pull`, rebuild with `make build`, and replace the binary on your `PATH`.

## Quickstart

Bind a key in `~/.tmux.conf` to launch cmdk in a tmux popup:

```tmux
bind-key Space display-popup -E "cmdk --pane-id=#{pane_id}"
```

Reload tmux (`tmux source-file ~/.tmux.conf`), then press `prefix + Space` inside any tmux session to open the launcher.

## Reference

For the full command reference, run `cmdk --help`. See also:

- `cmdk help exit-codes` - exit codes and error categories
- `cmdk config --provenance` - effective configuration and source provenance
- `cmdk attach [path]` - from outside tmux, attach to a configured or explicit cmdk-managed session
- `cmdk session resolve <path>` - inspect the planned session for a path
- `cmdk session window <path> --new` - create a background window in the cmdk-managed tmux session; add `--switch` to select it
- `cmdk window next` / `cmdk window previous` - cycle through tmux windows across sessions
- `cmdk docs` - configuration reference
- `cmdk icons` - supported icon aliases
