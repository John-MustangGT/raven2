// internal/monitoring/ssh_plugin.go
package monitoring

import (
    "bufio"
    "context"
    "fmt"
    "net"
    "strconv"
    "strings"
    "time"

    "raven2/internal/database"
)

// SSHPlugin connects to a TCP port and verifies the server sends a valid
// SSH identification banner (RFC 4253 4.2 requires "SSH-" as the first four
// bytes a server sends), without performing a key exchange or auth.
// Options:
//   port  int - default 22
type SSHPlugin struct{}

func (p *SSHPlugin) Name() string {
    return "ssh"
}

func (p *SSHPlugin) Execute(ctx context.Context, host *database.Host, check *database.Check) (*CheckResult, error) {
    target := targetAddress(host)
    if target == "" {
        return &CheckResult{ExitCode: 3, Output: "No IP address or hostname configured"}, nil
    }
    port := optInt(check.Options, "port", 22)
    addr := net.JoinHostPort(target, strconv.Itoa(port))

    var d net.Dialer
    start := time.Now()
    conn, err := d.DialContext(ctx, "tcp", addr)
    if err != nil {
        return &CheckResult{
            ExitCode: 2,
            Output:   fmt.Sprintf("SSH CRITICAL - could not connect to %s: %s", addr, err.Error()),
        }, nil
    }
    defer conn.Close()

    if deadline, ok := ctx.Deadline(); ok {
        conn.SetReadDeadline(deadline)
    } else {
        conn.SetReadDeadline(time.Now().Add(10 * time.Second))
    }

    banner, readErr := bufio.NewReader(conn).ReadString('\n')
    elapsed := time.Since(start)
    banner = strings.TrimRight(banner, "\r\n")

    if banner == "" {
        errMsg := "connection closed before a banner was received"
        if readErr != nil {
            errMsg = readErr.Error()
        }
        return &CheckResult{
            ExitCode: 2,
            Output:   fmt.Sprintf("SSH CRITICAL - connected to %s but received no banner: %s", addr, errMsg),
        }, nil
    }

    if !strings.HasPrefix(banner, "SSH-") {
        return &CheckResult{
            ExitCode: 1,
            Output:   fmt.Sprintf("SSH WARNING - connected to %s but banner is not a valid SSH identification string: %q", addr, banner),
        }, nil
    }

    return &CheckResult{
        ExitCode: 0,
        Output:   fmt.Sprintf("SSH OK - %s (%.3fs)", banner, elapsed.Seconds()),
        PerfData: fmt.Sprintf("time=%.3fs;;;0", elapsed.Seconds()),
    }, nil
}
