// internal/monitoring/tcp_plugin.go
package monitoring

import (
    "context"
    "fmt"
    "net"
    "strconv"
    "time"

    "raven2/internal/database"
)

// TCPPlugin checks that a TCP port accepts connections.
// Options:
//   port  int - required, the TCP port to connect to
type TCPPlugin struct{}

func (p *TCPPlugin) Name() string {
    return "tcp"
}

func (p *TCPPlugin) Execute(ctx context.Context, host *database.Host, check *database.Check) (*CheckResult, error) {
    target := targetAddress(host)
    if target == "" {
        return &CheckResult{ExitCode: 3, Output: "No IP address or hostname configured"}, nil
    }
    port := optInt(check.Options, "port", 0)
    if port <= 0 {
        return &CheckResult{ExitCode: 3, Output: "tcp check requires options.port"}, nil
    }
    addr := net.JoinHostPort(target, strconv.Itoa(port))

    var d net.Dialer
    start := time.Now()
    conn, err := d.DialContext(ctx, "tcp", addr)
    elapsed := time.Since(start)
    if err != nil {
        return &CheckResult{
            ExitCode: 2,
            Output:   fmt.Sprintf("TCP CRITICAL - could not connect to %s: %s", addr, err.Error()),
            PerfData: fmt.Sprintf("time=%.3fs;;;0", elapsed.Seconds()),
        }, nil
    }
    conn.Close()

    return &CheckResult{
        ExitCode: 0,
        Output:   fmt.Sprintf("TCP OK - connected to %s in %.3fs", addr, elapsed.Seconds()),
        PerfData: fmt.Sprintf("time=%.3fs;;;0", elapsed.Seconds()),
    }, nil
}
