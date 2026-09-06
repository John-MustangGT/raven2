// internal/monitoring/nagios_plugin.go
package monitoring

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os/exec"
    "strings"

    "raven2/internal/database"
)

// NagiosPlugin runs an external check binary that follows the Nagios/Icinga
// plugin API: https://nagios-plugins.org/doc/guidelines.html
//   - exit code 0/1/2/3 means OK/WARNING/CRITICAL/UNKNOWN
//   - stdout's first line is the short output, optionally followed by
//     "|perfdata"; any further lines are long output, each of which may
//     also carry its own trailing "|perfdata"
//
// Options (same names raven-discover has always generated and Configuration.md
// has always documented for this check type):
//
//	program  string   - required, path to the plugin binary
//	options  []string - extra arguments passed to the plugin
//	addhost  bool     - default true, prepends "-H <address>" so the plugin
//	                    knows its target without the config spelling out
//	                    $HOSTADDRESS$ itself
//	usedns   bool     - default false, use the hostname instead of the IP
//	                    for that -H value when both are available
//
// $HOSTADDRESS$/$HOSTNAME$ in any options entry are substituted too, for
// cases addhost doesn't cover (e.g. an argument that embeds the host).
//
// Registered under both "nagios" and "icinga", since Icinga plugins use the
// identical exit-code and output contract.
type NagiosPlugin struct{}

func (p *NagiosPlugin) Name() string {
    return "nagios"
}

func (p *NagiosPlugin) ValidateOptions(options map[string]interface{}) error {
    if optString(options, "program", "") == "" {
        return fmt.Errorf("nagios/icinga check requires options.program (path to the plugin binary)")
    }
    return nil
}

func (p *NagiosPlugin) Execute(ctx context.Context, host *database.Host, check *database.Check) (*CheckResult, error) {
    program := optString(check.Options, "program", "")
    if program == "" {
        return &CheckResult{
            ExitCode: 3,
            Output:   "nagios/icinga check requires options.program (path to the plugin binary)",
        }, nil
    }

    args := stringSliceOption(check.Options, "options")
    macros := map[string]string{
        "$HOSTADDRESS$": targetAddress(host),
        "$HOSTNAME$":    firstNonEmpty(host.Hostname, host.Name),
    }
    for i, arg := range args {
        for macro, value := range macros {
            arg = strings.ReplaceAll(arg, macro, value)
        }
        args[i] = arg
    }

    // addhost (default true) passes -H <address> the way every
    // monitoring-plugins/nagios-plugins check expects its target, so a
    // check doesn't have to spell out $HOSTADDRESS$ itself. usedns (default
    // false) picks the hostname over the IP when both are available.
    if optBool(check.Options, "addhost", true) {
        hostArg := host.IPv4
        if hostArg == "" || optBool(check.Options, "usedns", false) {
            hostArg = firstNonEmpty(host.Hostname, host.IPv4)
        }
        if hostArg != "" {
            args = append([]string{"-H", hostArg}, args...)
        }
    }

    cmd := exec.CommandContext(ctx, program, args...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    runErr := cmd.Run()

    if runErr != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
        return &CheckResult{
            ExitCode:   3,
            Output:     "Plugin execution timed out: " + program,
            LongOutput: stderr.String(),
        }, nil
    }

    exitCode := 0
    if runErr != nil {
        var exitErr *exec.ExitError
        if !errors.As(runErr, &exitErr) {
            // Could not even start the plugin (bad path, no exec permission, etc).
            return &CheckResult{
                ExitCode:   3,
                Output:     "Failed to execute plugin " + program + ": " + runErr.Error(),
                LongOutput: stderr.String(),
            }, nil
        }
        exitCode = exitErr.ExitCode()
    }
    if exitCode < 0 || exitCode > 3 {
        // Killed by a signal, or the plugin doesn't follow the spec.
        exitCode = 3
    }

    output, longOutput, perfData := parsePluginOutput(stdout.String())
    if output == "" {
        output = "Plugin produced no output"
        if stderr.Len() > 0 {
            longOutput = stderr.String()
        }
    }

    return &CheckResult{
        ExitCode:   exitCode,
        Output:     output,
        PerfData:   perfData,
        LongOutput: longOutput,
    }, nil
}

// parsePluginOutput splits Nagios/Icinga plugin stdout into the short
// output line, the joined long-output lines, and combined perfdata.
func parsePluginOutput(raw string) (output, longOutput, perfData string) {
    raw = strings.TrimRight(raw, "\n")
    if raw == "" {
        return "", "", ""
    }

    lines := strings.Split(raw, "\n")
    output, perfData = splitPerfData(lines[0])

    var longLines []string
    for _, line := range lines[1:] {
        text, perf := splitPerfData(line)
        longLines = append(longLines, text)
        if perf != "" {
            if perfData != "" {
                perfData += " "
            }
            perfData += perf
        }
    }
    longOutput = strings.Join(longLines, "\n")

    return output, longOutput, perfData
}

func splitPerfData(line string) (text, perf string) {
    if i := strings.Index(line, "|"); i >= 0 {
        return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
    }
    return strings.TrimSpace(line), ""
}

func stringSliceOption(opts map[string]interface{}, key string) []string {
    raw, ok := opts[key].([]interface{})
    if !ok {
        return nil
    }
    out := make([]string, 0, len(raw))
    for _, v := range raw {
        if s, ok := v.(string); ok {
            out = append(out, s)
        }
    }
    return out
}

func firstNonEmpty(values ...string) string {
    for _, v := range values {
        if v != "" {
            return v
        }
    }
    return ""
}
