// internal/monitoring/dns_plugin.go
package monitoring

import (
    "context"
    "fmt"
    "net"
    "strings"
    "time"

    "raven2/internal/database"
)

// DNSPlugin resolves a DNS record and optionally checks its value.
// Options:
//   query        string - name to resolve (default: the host's hostname)
//   record_type  string - A, AAAA, CNAME, MX, TXT, or NS (default: A)
//   server       string - DNS server to query directly, "host" or "host:port" (default: system resolver)
//   expected     string - if set, at least one returned record must contain this substring
type DNSPlugin struct{}

func (p *DNSPlugin) Name() string {
    return "dns"
}

func (p *DNSPlugin) Execute(ctx context.Context, host *database.Host, check *database.Check) (*CheckResult, error) {
    query := optString(check.Options, "query", firstNonEmpty(host.Hostname, host.Name))
    if query == "" {
        return &CheckResult{ExitCode: 3, Output: "dns check requires options.query or a host hostname"}, nil
    }
    recordType := strings.ToUpper(optString(check.Options, "record_type", "A"))
    expected := optString(check.Options, "expected", "")

    resolver := net.DefaultResolver
    if server := optString(check.Options, "server", ""); server != "" {
        addr := server
        if !strings.Contains(addr, ":") {
            addr += ":53"
        }
        resolver = &net.Resolver{
            PreferGo: true,
            Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
                d := net.Dialer{Timeout: 5 * time.Second}
                return d.DialContext(ctx, network, addr)
            },
        }
    }

    var records []string
    var lookupErr error

    switch recordType {
    case "A":
        ips, err := resolver.LookupIP(ctx, "ip4", query)
        lookupErr = err
        for _, ip := range ips {
            records = append(records, ip.String())
        }
    case "AAAA":
        ips, err := resolver.LookupIP(ctx, "ip6", query)
        lookupErr = err
        for _, ip := range ips {
            records = append(records, ip.String())
        }
    case "CNAME":
        cname, err := resolver.LookupCNAME(ctx, query)
        lookupErr = err
        if err == nil {
            records = []string{cname}
        }
    case "MX":
        mxs, err := resolver.LookupMX(ctx, query)
        lookupErr = err
        for _, mx := range mxs {
            records = append(records, fmt.Sprintf("%s (priority %d)", mx.Host, mx.Pref))
        }
    case "TXT":
        txts, err := resolver.LookupTXT(ctx, query)
        lookupErr = err
        records = txts
    case "NS":
        nss, err := resolver.LookupNS(ctx, query)
        lookupErr = err
        for _, ns := range nss {
            records = append(records, ns.Host)
        }
    default:
        return &CheckResult{ExitCode: 3, Output: "Unsupported dns record_type: " + recordType}, nil
    }

    if lookupErr != nil {
        return &CheckResult{
            ExitCode:   2,
            Output:     fmt.Sprintf("DNS CRITICAL - %s lookup for %s failed: %s", recordType, query, lookupErr.Error()),
            LongOutput: lookupErr.Error(),
        }, nil
    }
    if len(records) == 0 {
        return &CheckResult{
            ExitCode: 2,
            Output:   fmt.Sprintf("DNS CRITICAL - %s lookup for %s returned no records", recordType, query),
        }, nil
    }

    exitCode := 0
    status := "OK"
    if expected != "" {
        found := false
        for _, r := range records {
            if strings.Contains(r, expected) {
                found = true
                break
            }
        }
        if !found {
            exitCode = 1
            status = "WARNING"
        }
    }

    return &CheckResult{
        ExitCode:   exitCode,
        Output:     fmt.Sprintf("DNS %s - %s %s resolved to %d record(s)", status, query, recordType, len(records)),
        LongOutput: strings.Join(records, "\n"),
    }, nil
}
