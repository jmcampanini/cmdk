package generator

import "github.com/jmcampanini/cmdk/internal/item"

func NewRootGenerator(listWindows func() ([]item.Item, error)) GeneratorFunc {
	return func(accumulated []item.Item, ctx Context) []item.Item {
		windows, err := listWindows()
		if err != nil {
			return nil // TODO(M5): return error items instead of swallowing
		}
		return windows
	}
}
