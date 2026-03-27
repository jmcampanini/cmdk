# cmdk

Keyboard-driven tmux launcher. Runs as a TUI inside a tmux popup, presenting fuzzy-filterable lists of items for switching windows, opening directories, and running shell commands.

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
make install
```

## Usage

```
$ cmdk --help
Keyboard-driven tmux launcher

Usage:
  cmdk [flags]
  cmdk [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  config      Show resolved configuration and validate a config file
  docs        Show configuration reference
  help        Help about any command
  shorten     Apply display rules to shorten a path

Flags:
  -c, --config string    path to config file (also validates; exits 1 on error)
  -h, --help             help for cmdk
      --pane-id string   tmux pane ID
      --start-time int   process start time as epoch milliseconds (implies --timings)
      --theme string     color theme (light, dark)
      --timings          measure and print startup phase durations
      --timings-json     output timings as JSON (implies --timings)
  -v, --version          version for cmdk

Use "cmdk [command] --help" for more information about a command.
```
