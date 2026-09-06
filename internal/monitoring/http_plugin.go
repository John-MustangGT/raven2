// internal/monitoring/http_plugin.go
package monitoring

import (
    "context"
    "crypto/tls"
    "fmt"
    "net"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"

    "raven2/internal/database"
)

// HTTPPlugin connects to a host over HTTP or HTTPS, validates the TLS
// certificate for https, and checks the response status of a request
// (HEAD by default, so it doesn't pull the response body).
// Options:
//   scheme                string - "http" or "https" (default "http")
//   tls                    bool   - shorthand for scheme "https"
//   port                   int    - default 80, or 443 when scheme is https
//   path                   string - default "/"
//   method                 string - default "HEAD"
//   host_header            string - Host header / TLS SNI (default: host's hostname, else its address)
//   expected_status        int    - if set, only this exact status is OK; otherwise 2xx/3xx=OK, 4xx=WARNING, 5xx=CRITICAL
//   insecure_skip_verify   bool   - skip TLS certificate validation (default false)
type HTTPPlugin struct{}

func (p *HTTPPlugin) Name() string {
    return "http"
}

func (p *HTTPPlugin) Execute(ctx context.Context, host *database.Host, check *database.Check) (*CheckResult, error) {
    target := targetAddress(host)
    if target == "" {
        return &CheckResult{ExitCode: 3, Output: "No IP address or hostname configured"}, nil
    }

    scheme := strings.ToLower(optString(check.Options, "scheme", ""))
    if scheme == "" {
        scheme = "http"
        if check.Type == "https" {
            scheme = "https"
        }
    }
    if optBool(check.Options, "tls", false) {
        scheme = "https"
    }
    defaultPort := 80
    if scheme == "https" {
        defaultPort = 443
    }
    port := optInt(check.Options, "port", defaultPort)
    path := optString(check.Options, "path", "/")
    if !strings.HasPrefix(path, "/") {
        path = "/" + path
    }
    method := strings.ToUpper(optString(check.Options, "method", "HEAD"))
    insecure := optBool(check.Options, "insecure_skip_verify", false)

    reqHost := optString(check.Options, "host_header", firstNonEmpty(host.Hostname, target))

    reqURL := url.URL{
        Scheme: scheme,
        Host:   net.JoinHostPort(target, strconv.Itoa(port)),
        Path:   path,
    }

    req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), nil)
    if err != nil {
        return &CheckResult{ExitCode: 3, Output: "Failed to build request: " + err.Error()}, nil
    }
    req.Host = reqHost

    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                ServerName:         reqHost,
                InsecureSkipVerify: insecure,
            },
        },
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= 5 {
                return http.ErrUseLastResponse
            }
            return nil
        },
    }

    start := time.Now()
    resp, err := client.Do(req)
    elapsed := time.Since(start)
    if err != nil {
        // TLS certificate problems (expired, untrusted, hostname mismatch)
        // surface here as part of the handshake, so this also covers
        // "cert valid if https" without any separate check.
        return &CheckResult{
            ExitCode:   2,
            Output:     fmt.Sprintf("HTTP CRITICAL - %s %s failed: %s", method, reqURL.String(), err.Error()),
            LongOutput: err.Error(),
            PerfData:   fmt.Sprintf("time=%.3fs;;;0", elapsed.Seconds()),
        }, nil
    }
    defer resp.Body.Close()

    expected := optInt(check.Options, "expected_status", 0)
    exitCode := 0
    status := "OK"
    switch {
    case expected > 0:
        if resp.StatusCode != expected {
            exitCode = 2
            status = "CRITICAL"
        }
    case resp.StatusCode >= 500:
        exitCode = 2
        status = "CRITICAL"
    case resp.StatusCode >= 400:
        exitCode = 1
        status = "WARNING"
    }

    return &CheckResult{
        ExitCode: exitCode,
        Output:   fmt.Sprintf("HTTP %s - %s %s returned %d in %.3fs", status, method, reqURL.String(), resp.StatusCode, elapsed.Seconds()),
        PerfData: fmt.Sprintf("time=%.3fs;;;0", elapsed.Seconds()),
    }, nil
}
