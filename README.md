# cmdk

cmdk is a keyboard-driven tmux launcher: a fuzzy-filterable popup for switching windows, opening directories, and running configured commands. cmdk starts a shell only for commands written in the config, runs tmux, git, and zoxide without one, caps the output it reads from every subprocess, and kills a timed-out subprocess together with its process group.


## Install

cmdk distributes from HEAD only; there is no release channel or tagged binary.

Linux and macOS are supported. Linux is verified in required CI; other Unix-like systems are unverified, and Windows is unsupported.


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

With Git and [mise](https://mise.jdx.dev) installed (mise supplies the pinned Go toolchain from `mise.toml`):

```sh
git clone https://github.com/jmcampanini/cmdk.git
cd cmdk
mise install
mise run build
# Copy or symlink ./build/cmdk onto your PATH.
```


## Representative commands

| Invocation | Result |
| --- | --- |
| `cmdk --pane-id=#{pane_id}` | Open the launcher for a pane; the binding below runs it in a tmux popup. |
| `cmdk attach [path]` | Attach to a cmdk-managed session from outside tmux. |
| `cmdk session resolve <path>` | Inspect the planned session for a path. |
| `cmdk session window <path> --new` | Create a shell window in a cmdk-managed session. |
| `cmdk action run <exact-name>` | Run a configured session-window action. |
| `cmdk window next` / `cmdk window previous` | Cycle through tmux windows across sessions. |

`cmdk --help`, each command's `--help`, and the help topics `cmdk help exit-codes`, `cmdk docs`, and `cmdk icons` are the canonical reference.

Bind a key in `~/.tmux.conf` to open the launcher in a tmux popup, reload tmux with `tmux source-file ~/.tmux.conf`, then press `prefix + Space` inside any tmux session:

```tmux
bind-key Space display-popup -E "cmdk --pane-id=#{pane_id}"
```


## Required external programs

- `tmux` 3.2 or newer: the launcher and every tmux-backed command run it.
- `git`: resolving a repository, worktree, or worktree-container path to its shared managed session runs it.
- [`zoxide`](https://github.com/ajeetdsouza/zoxide), optional: zoxide-backed directory entries in the launcher come from it.
- `sh`: commands you configure (`cmd`, `launch_path_cmd`, and picker `source`) run through it.


## Configuration

cmdk reads `$XDG_CONFIG_HOME/cmdk/config.toml`, or `~/.config/cmdk/config.toml` when `XDG_CONFIG_HOME` is unset or empty. `cmdk config --help` shows how to inspect and validate the resolved configuration, and `cmdk docs` is the configuration reference.

A minimal config declares one action:

```toml
[[actions]]
name = "Editor"
matches = "dir"
cmd = "nvim"
```
