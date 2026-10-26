# cmdk

cmdk is a keyboard-driven tmux launcher: a fuzzy-filterable popup for switching windows, opening directories, and running configured commands. cmdk starts a shell only for commands written in the config, runs tmux, git, and zoxide without one, caps the output it reads from every subprocess, and kills a timed-out subprocess together with its process group.


## Install

cmdk supports versioned releases with prebuilt binaries and a Homebrew cask, plus a HEAD-only Homebrew formula built from source. Until the first release is published, use the HEAD formula or build from source.

Release archives target Linux and macOS on amd64 and arm64. Linux is verified in required CI; other Unix-like systems are unverified, and Windows is unsupported. Install the [required external programs](#required-external-programs) separately.


### Versioned releases

After the first release is published, install or upgrade with [mise](https://mise.jdx.dev) or the shared Homebrew tap:

```sh
mise use -g github:jmcampanini/cmdk
mise upgrade github:jmcampanini/cmdk

brew install --cask jmcampanini/tap/cmdk
brew upgrade --cask jmcampanini/tap/cmdk
```

Or download the archive for your OS and architecture from the [releases page](https://github.com/jmcampanini/cmdk/releases), extract it, and put `cmdk` on your `PATH`. To upgrade, download the new release and replace the binary. Each release includes SHA-256 checksums and GitHub build provenance attestations. On macOS, if a browser download is quarantined, run `xattr -d com.apple.quarantine cmdk` on the extracted binary before running it. The Homebrew cask handles quarantine removal and installs bash, zsh, and fish completions.


### Homebrew HEAD alternative (macOS)

The existing HEAD formula remains available for installs that track the latest commit:

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
mise trust
mise install
mise run build
# Copy or symlink ./build/cmdk onto your PATH.
```

To upgrade a source installation, pull the latest source, run `mise install` and `mise run build` again, and replace the binary on your `PATH`.


## Representative commands

| Invocation | Result |
| --- | --- |
| `cmdk --pane-id=#{pane_id}` | Open the launcher for a pane; the binding below runs it in a tmux popup. |
| `cmdk attach [path]` | Attach to a cmdk-managed session from outside tmux. |
| `cmdk session resolve <path>` | Inspect the planned session for a path. |
| `cmdk session window <path> --new` | Create a shell window in a cmdk-managed session. |
| `cmdk action run <exact-name>` | Run a configured session-window action. |
| `cmdk window next` / `cmdk window previous` | Cycle through tmux windows across sessions. |

`cmdk --help`, each command's `--help`, and the help topics `cmdk help exit-codes`, `cmdk help launching`, and `cmdk help templates` are the canonical reference.

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

cmdk reads `$XDG_CONFIG_HOME/cmdk/config.toml`, or `~/.config/cmdk/config.toml` when `XDG_CONFIG_HOME` is unset, empty, or not an absolute path. `cmdk config --help` is the configuration reference: every key with its meaning and constraints, and how to inspect and validate the resolved configuration.

A minimal config declares one action:

```toml
[[actions]]
name = "Editor"
matches = "dir"
cmd = "nvim"
```
