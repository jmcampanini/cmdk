package config

import "github.com/jmcampanini/go-config-loader/configloader"

// Overrides contains explicit command-line inputs, applied after the file.
// A nil field leaves the file or default value unchanged.
type Overrides struct {
	TruncationLength *int
	TruncationSymbol *string
}

func (o Overrides) load(cfg Config) (Config, configloader.LoadReport, error) {
	updates := make(configloader.Updates)
	if o.TruncationLength != nil {
		cfg.Display.TruncationLength = *o.TruncationLength
		updates["display.truncationlength"] = "--truncate"
	}
	if o.TruncationSymbol != nil {
		cfg.Display.TruncationSymbol = *o.TruncationSymbol
		updates["display.truncationsymbol"] = "--truncate-symbol"
	}
	return cfg, configloader.LoadReport{Updates: updates}, nil
}
