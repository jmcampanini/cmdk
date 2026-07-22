package actionrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/item"
)

// Prepared contains the selected action and context stack expected by
// execute.ResolveLaunch.
type Prepared struct {
	Selected    item.Item
	Accumulated []item.Item
}

// Prepare resolves a configured action and its noninteractive stage inputs
// without executing commands or interacting with tmux.
func Prepare(cfg config.Config, name, path string, rawInputs []string) (Prepared, error) {
	return PrepareWithPane(cfg, name, path, "", rawInputs)
}

func PrepareWithPane(cfg config.Config, name, path, paneID string, rawInputs []string) (Prepared, error) {
	action, err := ValidateAction(cfg, name)
	if err != nil {
		return Prepared{}, err
	}

	accumulated, err := baseContext(action.Matches, path)
	if err != nil {
		return Prepared{}, fmt.Errorf("action %q: %w", name, err)
	}

	provided, err := parseInputs(action.Stages, rawInputs)
	if err != nil {
		return Prepared{}, fmt.Errorf("action %q: %w", name, err)
	}

	acceptedInputs := acceptedInputKeys(action.Stages)
	for i, stage := range action.Stages {
		value, supplied := provided[stage.Key]
		if !supplied {
			switch stage.Type {
			case "prompt":
				if stage.Default == "" {
					return Prepared{}, fmt.Errorf("action %q stage %d input %q is required: prompt has no default (accepted inputs: %s)", name, i, stage.Key, acceptedInputs)
				}
				data := execute.FlattenData(accumulated)
				if paneID != "" {
					data["pane_id"] = paneID
				}
				value, err = execute.RenderCmd(stage.Default, data)
				if err != nil {
					return Prepared{}, fmt.Errorf("action %q stage %d input %q default template: %w", name, i, stage.Key, err)
				}
			case "picker":
				return Prepared{}, fmt.Errorf("action %q stage %d input %q is required: picker input must be supplied (accepted inputs: %s)", name, i, stage.Key, acceptedInputs)
			default:
				return Prepared{}, fmt.Errorf("action %q stage %d input %q has unsupported type %q", name, i, stage.Key, stage.Type)
			}
		}

		if strings.TrimSpace(value) == "" && (stage.Type != "prompt" || !stage.AllowEmpty) {
			return Prepared{}, fmt.Errorf("action %q stage %d input %q cannot be empty (accepted inputs: %s)", name, i, stage.Key, acceptedInputs)
		}

		accumulated = append(accumulated, item.Item{
			Type:    "stage-result",
			Display: value,
			Data:    map[string]string{stage.Key: value},
		})
	}

	return Prepared{
		Selected:    action.ToItem(),
		Accumulated: accumulated,
	}, nil
}

func ValidateAction(cfg config.Config, name string) (config.Action, error) {
	action, err := findAction(cfg.Actions, name)
	if err != nil {
		return config.Action{}, err
	}
	if action.Matches == "session" {
		return config.Action{}, fmt.Errorf("action %q matches session; noninteractive session actions are not supported", name)
	}

	hasLaunchPath := action.LaunchPath != "" || action.LaunchPathCmd != ""
	mode := config.EffectiveLaunchMode(action.Matches, action.LaunchMode, hasLaunchPath)
	if mode != config.LaunchModeSessionWindow {
		return config.Action{}, fmt.Errorf("action %q has effective launch_mode %q; only %q is supported", name, mode, config.LaunchModeSessionWindow)
	}
	return action, nil
}

func findAction(actions []config.Action, name string) (config.Action, error) {
	matches := make([]int, 0, 1)
	for i := range actions {
		if actions[i].Name == name {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return config.Action{}, fmt.Errorf("configured action %q not found", name)
	}
	if len(matches) > 1 {
		details := make([]string, len(matches))
		for i, index := range matches {
			details[i] = fmt.Sprintf("actions[%d] matches=%q", index, actions[index].Matches)
		}
		return config.Action{}, fmt.Errorf("configured action name %q is ambiguous: %d exact matches (%s)", name, len(matches), strings.Join(details, ", "))
	}
	return actions[matches[0]], nil
}

func baseContext(matchType, path string) ([]item.Item, error) {
	switch matchType {
	case "root":
		if path != "" {
			return nil, fmt.Errorf("--path is not valid for an action matching root (got %q)", path)
		}
		return nil, nil
	case "dir":
		if path == "" {
			return nil, fmt.Errorf("--path is required for an action matching dir")
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve --path %q: %w", path, err)
		}
		absolute = filepath.Clean(absolute)
		if !utf8.ValidString(absolute) {
			return nil, fmt.Errorf("--path is not valid UTF-8")
		}
		if strings.ContainsFunc(absolute, unicode.IsControl) {
			return nil, fmt.Errorf("--path %q contains control characters", path)
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return nil, fmt.Errorf("validate --path %q: %w", path, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("--path %q is not a directory", path)
		}
		return []item.Item{{
			Type:    "dir",
			Display: absolute,
			Action:  item.ActionNextList,
			Data:    map[string]string{"path": absolute},
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported action matches %q", matchType)
	}
}

func parseInputs(stages []config.StageConfig, rawInputs []string) (map[string]string, error) {
	accepted := make(map[string]struct{}, len(stages))
	for _, stage := range stages {
		accepted[stage.Key] = struct{}{}
	}
	keyList := acceptedInputKeys(stages)

	provided := make(map[string]string, len(rawInputs))
	for _, raw := range rawInputs {
		key, value, found := strings.Cut(raw, "=")
		if !found {
			return nil, fmt.Errorf("input %q must use key=value (accepted keys: %s)", raw, keyList)
		}
		if key == "" {
			return nil, fmt.Errorf("input key cannot be empty (accepted keys: %s)", keyList)
		}
		if _, ok := accepted[key]; !ok {
			return nil, fmt.Errorf("unknown input key %q (accepted keys: %s)", key, keyList)
		}
		if _, exists := provided[key]; exists {
			return nil, fmt.Errorf("input key %q was supplied more than once (accepted keys: %s)", key, keyList)
		}
		provided[key] = value
	}
	return provided, nil
}

func acceptedInputKeys(stages []config.StageConfig) string {
	if len(stages) == 0 {
		return "(none)"
	}
	keys := make([]string, len(stages))
	for i, stage := range stages {
		keys[i] = stage.Key
	}
	return strings.Join(keys, ", ")
}
