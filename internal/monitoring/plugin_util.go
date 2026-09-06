// internal/monitoring/plugin_util.go
package monitoring

import (
    "strconv"

    "raven2/internal/database"
)

// targetAddress picks the address a check should connect to: the host's
// IPv4 if set, falling back to its hostname.
func targetAddress(host *database.Host) string {
    if host.IPv4 != "" {
        return host.IPv4
    }
    return host.Hostname
}

// Check.Options round-trips through JSON in BoltDB (see boltstore.go), so a
// number entered as YAML int comes back out as float64; these helpers
// normalize that instead of every plugin re-deriving it.

func optString(opts map[string]interface{}, key, def string) string {
    if opts == nil {
        return def
    }
    if s, ok := opts[key].(string); ok && s != "" {
        return s
    }
    return def
}

func optInt(opts map[string]interface{}, key string, def int) int {
    if opts == nil {
        return def
    }
    switch v := opts[key].(type) {
    case int:
        return v
    case int64:
        return int(v)
    case float64:
        return int(v)
    case string:
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return def
}

func optBool(opts map[string]interface{}, key string, def bool) bool {
    if opts == nil {
        return def
    }
    if b, ok := opts[key].(bool); ok {
        return b
    }
    return def
}
