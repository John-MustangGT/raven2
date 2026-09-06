// internal/monitoring/validate.go
package monitoring

import (
    "fmt"

    "raven2/internal/config"
)

// ValidateConfig checks that every check references a known plugin type and
// that the plugin accepts the check's options, without touching the
// database, network, or filesystem beyond what config.Load already did.
// It's what `raven -test` and ValidateConfigFile run, and is meant to be
// safe to call at any time - including while another raven instance is
// already running against the same database - since it never opens one.
func ValidateConfig(cfg *config.Config) []error {
    var errs []error
    plugins := newPluginRegistry()

    for _, check := range cfg.Checks {
        plugin, exists := plugins[check.Type]
        if !exists {
            errs = append(errs, fmt.Errorf("check %q: unknown type %q", check.ID, check.Type))
            continue
        }

        if validator, ok := plugin.(OptionsValidator); ok {
            if err := validator.ValidateOptions(check.Options); err != nil {
                errs = append(errs, fmt.Errorf("check %q (%s): %w", check.ID, check.Type, err))
            }
        }
    }

    return errs
}
