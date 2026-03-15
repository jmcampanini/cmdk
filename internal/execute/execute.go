package execute

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
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

func Run(accumulated []item.Item, selected item.Item, execFn ExecFn) error {
	if selected.Cmd == "" {
		return fmt.Errorf("selected item has no command to execute (display: %q)", selected.Display)
	}
	all := slices.Concat(accumulated, []item.Item{selected})
	data := FlattenData(all)

	rendered, err := RenderCmd(selected.Cmd, data)
	if err != nil {
		return err
	}

	shPath, err := exec.LookPath("sh")
	if err != nil {
		return err
	}

	return execFn(shPath, []string{"sh", "-c", rendered}, os.Environ())
}
