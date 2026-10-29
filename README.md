# cmdk

cmdk is a keyboard-driven tmux launcher: a fuzzy-filterable popup for switching windows, opening directories, and running configured commands. cmdk starts a shell only for commands written in the config, runs tmux, git, and zoxide without one, caps the output it reads from every subprocess, and kills a timed-out subprocess together with its process group.


## Install

cmdk supports versioned releases for Linux and macOS on amd64 and arm64, with prebuilt binaries and a Homebrew cask, plus a HEAD-only Homebrew formula; until the first release, install the HEAD formula or build from source.

After the first release, install or upgrade with [mise](https://mise.jdx.dev):

```sh
mise use -g github:jmcampanini/cmdk
mise upgrade github:jmcampanini/cmdk
```

After the first release, install or upgrade the cask with Homebrew:

```sh
brew install --cask jmcampanini/tap/cmdk
brew upgrade --cask jmcampanini/tap/cmdk
```

After the first release, download the matching archive from [GitHub Releases](https://github.com/jmcampanini/cmdk/releases), then install or upgrade with `tar` and administrator access through `sudo`:

```sh
tar -xzf 'cmdk_<version>_<os>_<arch>.tar.gz'
# macOS only, if the browser quarantined the downloaded binary:
xattr -d com.apple.quarantine cmdk
sudo install -d /usr/local/bin
sudo install -m 755 cmdk /usr/local/bin/cmdk
```

To track the latest commit on macOS, install or upgrade the HEAD formula with Homebrew:

```sh
brew tap jmcampanini/cmdk https://github.com/jmcampanini/cmdk
brew install --HEAD jmcampanini/cmdk/cmdk
brew upgrade --fetch-HEAD cmdk
```

To build from source with Git, [mise](https://mise.jdx.dev), and administrator access through `sudo`, clone, build, and install; run the final five commands again to upgrade:

```sh
git clone https://github.com/jmcampanini/cmdk.git
cd cmdk
mise trust
git pull --ff-only
mise install
mise run build
sudo install -d /usr/local/bin
sudo install -m 755 build/cmdk /usr/local/bin/cmdk
```


## Representative commands

| Invocation | Result |
| --- | --- |
| `cmdk --pane-id=#{pane_id}` | Open the launcher for a pane; `cmdk --help` shows a tmux popup binding. |
| `cmdk attach [path]` | Attach to a cmdk-managed session from outside tmux. |
| `cmdk session resolve <path>` | Inspect the planned session for a path. |
| `cmdk session window <path> --new` | Create a shell window in a cmdk-managed session. |
| `cmdk action run <exact-name>` | Run a configured session-window action. |
| `cmdk window next` / `cmdk window previous` | Cycle through tmux windows across sessions. |

`cmdk --help`, each command's `--help`, and the help topics `cmdk help exit-codes`, `cmdk help launching`, and `cmdk help templates` are the canonical reference.


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
