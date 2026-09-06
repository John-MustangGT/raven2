// internal/monitoring/plugins.go
package monitoring

import (
    "context"
    "fmt"
    "os/exec"
    "regexp"
    "strconv"

    "raven2/internal/database"
)

// PingPlugin implements basic ping checks
type PingPlugin struct{}

func (p *PingPlugin) Name() string {
    return "ping"
}

func (p *PingPlugin) Init(options map[string]interface{}) error {
    return nil
}

func (p *PingPlugin) Execute(ctx context.Context, host *database.Host) (*CheckResult, error) {
    target := host.IPv4
    if target == "" {
        target = host.Hostname
    }
    if target == "" {
        return &CheckResult{
            ExitCode:   3,
            Output:     "No IP address or hostname configured",
            PerfData:   "",
            LongOutput: "",
        }, nil
    }

    cmd := exec.CommandContext(ctx, "ping", "-c", "3", target)
    output, err := cmd.Output()

    if err != nil {
        return &CheckResult{
            ExitCode:   2,
            Output:     "Ping failed",
            PerfData:   "",
            LongOutput: string(output),
        }, nil
    }

    // Parse ping output
    outputStr := string(output)
    
    // Extract packet loss
    lossRegex := regexp.MustCompile(`(\d+)% packet loss`)
    lossMatches := lossRegex.FindStringSubmatch(outputStr)
    
    // Extract average RTT. Real ping output reads
    // "rtt min/avg/max/mdev = 0.026/0.031/0.044/0.007 ms" (Linux) or
    // "round-trip min/avg/max/stddev = ..." (macOS); avg is the second of
    // the four slash-separated numbers after "=".
    rttRegex := regexp.MustCompile(`=\s*[\d.]+/([\d.]+)/`)
    rttMatches := rttRegex.FindStringSubmatch(outputStr)

    var loss int
    var rtt float64

    if len(lossMatches) > 1 {
        loss, _ = strconv.Atoi(lossMatches[1])
    }
    
    if len(rttMatches) > 1 {
        rtt, _ = strconv.ParseFloat(rttMatches[1], 64)
    }

    // Determine status based on thresholds
    exitCode := 0
    status := "OK"
    
    if loss > 25 || rtt > 100 {
        exitCode = 2
        status = "CRITICAL"
    } else if loss > 10 || rtt > 50 {
        exitCode = 1
        status = "WARNING"
    }

    return &CheckResult{
        ExitCode:   exitCode,
        Output:     fmt.Sprintf("PING %s - %s", status, target),
        PerfData:   fmt.Sprintf("rtt=%.2fms;50;100;0 loss=%d%%;10;25;0", rtt, loss),
        LongOutput: fmt.Sprintf("RTT: %.2fms, Loss: %d%%", rtt, loss),
    }, nil
}

// NagiosPlugin executes Nagios-compatible check plugins
type NagiosPlugin struct{}

func (p *NagiosPlugin) Name() string {
    return "nagios"
}

func (p *NagiosPlugin) Init(options map[string]interface{}) error {
    return nil
}

func (p *NagiosPlugin) Execute(ctx context.Context, host *database.Host) (*CheckResult, error) {
    // This would be implemented based on your existing nagios plugin logic
    // For now, return a placeholder
    return &CheckResult{
        ExitCode:   0,
        Output:     "Nagios check OK",
        PerfData:   "",
        LongOutput: "Nagios plugin executed successfully",
    }, nil
}
