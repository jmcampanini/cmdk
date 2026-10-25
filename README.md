# cmdk

cmdk is a keyboard-driven tmux launcher: a fuzzy-filterable popup for switching windows, opening directories, and running configured commands. Only commands written in the config go through a shell; cmdk runs tmux, git, and zoxide with fixed arguments, caps the output it reads from every subprocess, and kills a timed-out subprocess together with its process group.


## Install

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

To upgrade, run `git pull`, rebuild with `mise install && mise run build`, and replace the binary on your `PATH`.


## Representative commands

| Invocation | Result |
| --- | --- |
| `cmdk --pane-id=#{pane_id}` | Open the launcher in a tmux popup for the current pane. |
| `cmdk attach [path]` | Attach to a cmdk-managed session. |
| `cmdk session resolve <path>` | Inspect the planned session for a path. |
| `cmdk session window <path> --new` | Create a window in a cmdk-managed session. |
| `cmdk action run <exact-name>` | Run a configured session-window action. |
| `cmdk window next` / `cmdk window previous` | Cycle through tmux windows across sessions. |

`cmdk --help`, each command's `--help`, and the help topics `cmdk help exit-codes`, `cmdk docs`, and `cmdk icons` are the canonical reference.

The launcher is the usual entry point. Bind a key in `~/.tmux.conf` to open it in a tmux popup, reload tmux with `tmux source-file ~/.tmux.conf`, then press `prefix + Space` inside any tmux session:

```tmux
bind-key Space display-popup -E "cmdk --pane-id=#{pane_id}"
```


## Required external programs

- `tmux` 3.2 or newer: the launcher and every tmux-backed command run it.
- `git`: resolving a directory inside a Git repository or worktree to its shared managed session runs it; directories outside a repository never need it.
- [`zoxide`](https://github.com/ajeetdsouza/zoxide), optional: zoxide-backed directory entries in the launcher come from it.
- `sh`: commands you configure (`cmd`, `launch_path_cmd`, and picker `source`) run through it.


## Configuration

cmdk reads `$XDG_CONFIG_HOME/cmdk/config.toml`, or `~/.config/cmdk/config.toml` when `XDG_CONFIG_HOME` is unset. `cmdk config --help` shows how to inspect and validate the resolved configuration, and `cmdk docs` is the configuration reference.

A minimal config declares one action:

```toml
[[actions]]
name = "Editor"
matches = "dir"
cmd = "nvim"
```
