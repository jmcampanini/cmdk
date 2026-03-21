package execute

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/jmcampanini/cmdk/internal/item"
)

type ExecFn func(argv0 string, argv []string, envv []string) error

var tmplFuncs = template.FuncMap{
	"sq": func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	},
}

func RenderCmd(cmdTemplate string, data map[string]string) (string, error) {
	tmpl, err := template.New("cmd").Funcs(tmplFuncs).Option("missingkey=error").Parse(cmdTemplate)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func FlattenData(accumulated []item.Item) map[string]string {
	merged := make(map[string]string)
	for _, it := range accumulated {
		maps.Copy(merged, it.Data)
	}
	return merged
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]`)

func NormalizeKey(key string) string {
	return "CMDK_" + strings.ToUpper(nonAlphaNum.ReplaceAllString(key, "_"))
}

func BuildCMDKEnvVars(accumulated []item.Item, paneID string) []string {
	flat := FlattenData(accumulated)
	normalized := make(map[string]string, len(flat)+1)
	for k, v := range flat {
		normalized[NormalizeKey(k)] = v
	}
	if paneID != "" {
		normalized["CMDK_PANE_ID"] = paneID
	}
	envs := make([]string, 0, len(normalized))
	for k, v := range normalized {
		envs = append(envs, k+"="+v)
	}
	return envs
}

func Run(accumulated []item.Item, selected item.Item, paneID string, execFn ExecFn) error {
	if selected.Cmd == "" {
		return fmt.Errorf("selected item has no command to execute (display: %q)", selected.Display)
	}
	all := slices.Concat(accumulated, []item.Item{selected})
	data := FlattenData(all)
	if paneID != "" {
		data["pane_id"] = paneID
	}

	rendered, err := RenderCmd(selected.Cmd, data)
	if err != nil {
		return err
	}

	shPath, err := exec.LookPath("sh")
	if err != nil {
		return err
	}

	base := slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, "CMDK_")
	})
	envv := slices.Concat(base, BuildCMDKEnvVars(all, paneID))
	return execFn(shPath, []string{"sh", "-c", rendered}, envv)
}
