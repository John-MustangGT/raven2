// cmd/raven-discover/main.go
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// Nmap XML structures
type NmapRun struct {
	XMLName   xml.Name `xml:"nmaprun"`
	Scanner   string   `xml:"scanner,attr"`
	Args      string   `xml:"args,attr"`
	Start     int64    `xml:"start,attr"`
	StartStr  string   `xml:"startstr,attr"`
	Version   string   `xml:"version,attr"`
	ScanInfo  ScanInfo `xml:"scaninfo"`
	Hosts     []Host   `xml:"host"`
}

type ScanInfo struct {
	Type        string `xml:"type,attr"`
	Protocol    string `xml:"protocol,attr"`
	NumServices int    `xml:"numservices,attr"`
	Services    string `xml:"services,attr"`
}

type Host struct {
	StartTime int64       `xml:"starttime,attr"`
	EndTime   int64       `xml:"endtime,attr"`
	Status    HostStatus  `xml:"status"`
	Addresses []Address   `xml:"address"`
	Hostnames []Hostname  `xml:"hostnames>hostname"`
	Ports     []Port      `xml:"ports>port"`
	OS        []OSMatch   `xml:"os>osmatch"`
}

type HostStatus struct {
	State     string `xml:"state,attr"`
	Reason    string `xml:"reason,attr"`
	ReasonTTL int    `xml:"reason_ttl,attr"`
}

type Address struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type Hostname struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type Port struct {
	Protocol string      `xml:"protocol,attr"`
	PortID   int         `xml:"portid,attr"`
	State    PortState   `xml:"state"`
	Service  PortService `xml:"service"`
}

type PortState struct {
	State     string `xml:"state,attr"`
	Reason    string `xml:"reason,attr"`
	ReasonTTL int    `xml:"reason_ttl,attr"`
}

type PortService struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
	Method  string `xml:"method,attr"`
	Conf    int    `xml:"conf,attr"`
}

type OSMatch struct {
	Name     string `xml:"name,attr"`
	Accuracy int    `xml:"accuracy,attr"`
}

// Raven configuration structures
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Logging    LoggingConfig    `yaml:"logging"`
	Hosts      []HostConfig     `yaml:"hosts"`
	Checks     []CheckConfig    `yaml:"checks"`
}

type ServerConfig struct {
	Port         string `yaml:"port"`
	Workers      int    `yaml:"workers"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	Type              string `yaml:"type"`
	Path              string `yaml:"path"`
	BackupInterval    string `yaml:"backup_interval"`
	CleanupInterval   string `yaml:"cleanup_interval"`
	HistoryRetention  string `yaml:"history_retention"`
	CompactInterval   string `yaml:"compact_interval"`
}

type PrometheusConfig struct {
	Enabled     bool   `yaml:"enabled"`
	MetricsPath string `yaml:"metrics_path"`
	PushGateway string `yaml:"push_gateway"`
}

type MonitoringConfig struct {
	DefaultInterval string `yaml:"default_interval"`
	MaxRetries      int    `yaml:"max_retries"`
	Timeout         string `yaml:"timeout"`
	BatchSize       int    `yaml:"batch_size"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type HostConfig struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	DisplayName string            `yaml:"display_name"`
	IPv4        string            `yaml:"ipv4"`
	Hostname    string            `yaml:"hostname"`
	Group       string            `yaml:"group"`
	Enabled     bool              `yaml:"enabled"`
	Tags        map[string]string `yaml:"tags"`
}

type CheckConfig struct {
	ID        string                   `yaml:"id"`
	Name      string                   `yaml:"name"`
	Type      string                   `yaml:"type"`
	Hosts     []string                 `yaml:"hosts"`
	Interval  map[string]string        `yaml:"interval"`
	Threshold int                      `yaml:"threshold"`
	Timeout   string                   `yaml:"timeout"`
	Enabled   bool                     `yaml:"enabled"`
	Options   map[string]interface{}   `yaml:"options"`
}

// Port service mapping for check generation
var serviceChecks = map[int]CheckTemplate{
	22: {
		Type:    "nagios",
		Name:    "SSH Service",
		Timeout: "10s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_ssh",
			"options": []string{"-4"},
		},
	},
	23: {
		Type:    "nagios",
		Name:    "Telnet Service",
		Timeout: "10s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_tcp",
			"options": []string{"-p", "23"},
		},
	},
	25: {
		Type:    "nagios",
		Name:    "SMTP Service",
		Timeout: "10s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_smtp",
			"options": []string{},
		},
	},
	80: {
		Type:    "nagios",
		Name:    "HTTP Service",
		Timeout: "15s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_http",
			"options": []string{"-v"},
		},
	},
	123: {
		Type:    "nagios",
		Name:    "NTP Service",
		Timeout: "10s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_ntp",
			"options": []string{},
		},
	},
	161: {
		Type:    "nagios",
		Name:    "SNMP Service",
		Timeout: "10s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_snmp",
			"options": []string{"-C", "public", "-o", "1.3.6.1.2.1.1.1.0"},
		},
	},
	162: {
		Type:    "nagios",
		Name:    "SNMP Trap Service",
		Timeout: "10s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_tcp",
			"options": []string{"-p", "162", "-u"},
		},
	},
	443: {
		Type:    "nagios",
		Name:    "HTTPS Service",
		Timeout: "15s",
		Options: map[string]interface{}{
			"program": "/usr/lib/nagios/plugins/check_http",
			"options": []string{"-S", "-C", "30,15"},
		},
	},
}

type CheckTemplate struct {
	Type    string
	Name    string
	Timeout string
	Options map[string]interface{}
}

func main() {
	var (
		network     = flag.String("network", "", "CIDR network to scan (e.g., 192.168.1.0/24)")
		xmlFile     = flag.String("xml", "", "Use existing nmap XML file instead of scanning")
		output      = flag.String("output", "config.yaml", "Output configuration file")
		group       = flag.String("group", "discovered", "Group name for discovered hosts")
		dhcpRange   = flag.String("dhcp", "100-200", "DHCP range (e.g., 100-200) - hosts in this range won't have static IP configured")
		nmapPath    = flag.String("nmap", "/usr/bin/nmap", "Path to nmap binary")
		enabled     = flag.Bool("enabled", true, "Mark discovered hosts as enabled")
		osDetection = flag.Bool("os", false, "Enable OS detection (requires root)")
		verbose     = flag.Bool("verbose", false, "Verbose output")
	)
	flag.Parse()

	if *network == "" && *xmlFile == "" {
		// Try to detect local network
		detected := detectLocalNetwork()
		if detected == "" {
			log.Fatal("No network specified and couldn't detect local network. Use -network flag.")
		}
		*network = detected
		fmt.Printf("Auto-detected network: %s\n", *network)
	}

	var nmapData []byte
	var err error

	if *xmlFile != "" {
		fmt.Printf("Reading nmap XML from: %s\n", *xmlFile)
		nmapData, err = os.ReadFile(*xmlFile)
		if err != nil {
			log.Fatalf("Failed to read XML file: %v", err)
		}
	} else {
		fmt.Printf("Scanning network: %s\n", *network)
		nmapData, err = runNmapScan(*network, *nmapPath, *osDetection, *verbose)
		if err != nil {
			log.Fatalf("Failed to run nmap: %v", err)
		}
	}

	// Parse nmap XML
	var nmapRun NmapRun
	if err := xml.Unmarshal(nmapData, &nmapRun); err != nil {
		log.Fatalf("Failed to parse nmap XML: %v", err)
	}

	// Parse DHCP range
	dhcpLow, dhcpHigh := parseDHCPRange(*dhcpRange)

	// Generate configuration
	config := generateConfig(&nmapRun, *group, dhcpLow, dhcpHigh, *enabled)

	// Write configuration
	if err := writeConfig(config, *output); err != nil {
		log.Fatalf("Failed to write configuration: %v", err)
	}

	fmt.Printf("\nConfiguration written to: %s\n", *output)
	fmt.Printf("Discovered %d hosts and generated %d checks\n", len(config.Hosts), len(config.Checks))
}

func detectLocalNetwork() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				if ipnet.IP.IsGlobalUnicast() {
					return ipnet.String()
				}
			}
		}
	}
	return ""
}

func runNmapScan(network, nmapPath string, osDetection, verbose bool) ([]byte, error) {
	args := []string{
		"--system-dns",
		"-oX", "-",
		"-p", "22,23,25,80,123,161,162,443",
	}

	if osDetection {
		args = append(args, "-O")
	}

	if verbose {
		args = append(args, "-v")
	}

	args = append(args, network)

	fmt.Printf("Running: %s %s\n", nmapPath, strings.Join(args, " "))

	cmd := exec.Command(nmapPath, args...)
	output, err := cmd.Output()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				return nil, fmt.Errorf("nmap exited with status %d", status.ExitStatus())
			}
		}
		return nil, fmt.Errorf("nmap execution failed: %v", err)
	}

	return output, nil
}

func parseDHCPRange(dhcpRange string) (int, int) {
	parts := strings.Split(dhcpRange, "-")
	if len(parts) != 2 {
		return 100, 200 // Default range
	}

	low, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	high, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

	if err1 != nil || err2 != nil {
		return 100, 200 // Default range
	}

	return low, high
}

func generateConfig(nmapRun *NmapRun, group string, dhcpLow, dhcpHigh int, enabled bool) *Config {
	config := &Config{
		Server: ServerConfig{
			Port:         ":8000",
			Workers:      3,
			ReadTimeout:  "30s",
			WriteTimeout: "30s",
		},
		Database: DatabaseConfig{
			Type:              "boltdb",
			Path:              "./data/raven.db",
			BackupInterval:    "24h",
			CleanupInterval:   "1h",
			HistoryRetention:  "720h", // 30 days
			CompactInterval:   "24h",
		},
		Prometheus: PrometheusConfig{
			Enabled:     true,
			MetricsPath: "/metrics",
			PushGateway: "",
		},
		Monitoring: MonitoringConfig{
			DefaultInterval: "5m",
			MaxRetries:      3,
			Timeout:         "30s",
			BatchSize:       10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	var hosts []HostConfig
	portHosts := make(map[int][]string)
	allHosts := make([]string, 0)

	// Process discovered hosts
	for _, host := range nmapRun.Hosts {
		if host.Status.State != "up" {
			continue
		}

		hostConfig := processHost(host, group, dhcpLow, dhcpHigh, enabled)
		if hostConfig != nil {
			hosts = append(hosts, *hostConfig)
			allHosts = append(allHosts, hostConfig.ID)

			// Track which hosts have which ports open
			for _, port := range host.Ports {
				if port.State.State == "open" {
					portHosts[port.PortID] = append(portHosts[port.PortID], hostConfig.ID)
				}
			}
		}
	}

	config.Hosts = hosts

	// Generate checks
	var checks []CheckConfig

	// Add ping check for all hosts
	if len(allHosts) > 0 {
		pingCheck := CheckConfig{
			ID:   "ping-check",
			Name: "Ping Check",
			Type: "ping",
			Hosts: allHosts,
			Interval: map[string]string{
				"ok":       "5m",
				"warning":  "2m",
				"critical": "1m",
				"unknown":  "1m",
			},
			Threshold: 3,
			Timeout:   "10s",
			Enabled:   true,
			Options: map[string]interface{}{
				"count": "3",
			},
		}
		checks = append(checks, pingCheck)
	}

	// Generate port-specific checks
	var ports []int
	for port := range portHosts {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	for _, port := range ports {
		hostList := portHosts[port]
		if len(hostList) == 0 {
			continue
		}

		checkTemplate, exists := serviceChecks[port]
		if !exists {
			// Generic TCP check for unknown ports
			checkTemplate = CheckTemplate{
				Type:    "nagios",
				Name:    fmt.Sprintf("Port %d Check", port),
				Timeout: "10s",
				Options: map[string]interface{}{
					"program": "/usr/lib/nagios/plugins/check_tcp",
					"options": []string{"-p", strconv.Itoa(port)},
				},
			}
		}

		portCheck := CheckConfig{
			ID:   fmt.Sprintf("port-%d-check", port),
			Name: fmt.Sprintf("%s (Port %d)", checkTemplate.Name, port),
			Type: checkTemplate.Type,
			Hosts: hostList,
			Interval: map[string]string{
				"ok":       "15m",
				"warning":  "5m",
				"critical": "2m",
				"unknown":  "2m",
			},
			Threshold: 2,
			Timeout:   checkTemplate.Timeout,
			Enabled:   true,
			Options:   checkTemplate.Options,
		}
		checks = append(checks, portCheck)
	}

	config.Checks = checks
	return config
}

func processHost(host Host, group string, dhcpLow, dhcpHigh int, enabled bool) *HostConfig {
	var ipv4, hostname string

	// Get IP address
	for _, addr := range host.Addresses {
		if addr.AddrType == "ipv4" {
			ipv4 = addr.Addr
			break
		}
	}

	if ipv4 == "" {
		return nil
	}

	// Get hostname
	for _, hn := range host.Hostnames {
		if hn.Type == "PTR" || hn.Type == "user" {
			hostname = hn.Name
			break
		}
	}

	// Generate host ID and display name
	hostID := generateHostID(ipv4, hostname)
	displayName := hostID
	if hostname != "" {
		displayName = strings.Split(hostname, ".")[0]
	}

	// Check if IP is in DHCP range
	isDHCP := isInDHCPRange(ipv4, dhcpLow, dhcpHigh)

	tags := make(map[string]string)
	
	// Add OS information if available
	if len(host.OS) > 0 && host.OS[0].Name != "" {
		tags["os"] = host.OS[0].Name
		tags["os_accuracy"] = strconv.Itoa(host.OS[0].Accuracy)
	}

	// Add port information
	var openPorts []string
	for _, port := range host.Ports {
		if port.State.State == "open" {
			openPorts = append(openPorts, strconv.Itoa(port.PortID))
		}
	}
	if len(openPorts) > 0 {
		tags["open_ports"] = strings.Join(openPorts, ",")
	}

	// Add discovery timestamp
	tags["discovered"] = time.Now().Format(time.RFC3339)

	hostConfig := &HostConfig{
		ID:          hostID,
		Name:        displayName,
		DisplayName: displayName,
		Group:       group,
		Enabled:     enabled,
		Tags:        tags,
	}

	// Only set static IP if not in DHCP range
	if !isDHCP {
		hostConfig.IPv4 = ipv4
	}

	if hostname != "" {
		hostConfig.Hostname = hostname
	}

	return hostConfig
}

func generateHostID(ipv4, hostname string) string {
	if hostname != "" {
		// Use first part of hostname
		parts := strings.Split(hostname, ".")
		return strings.ToLower(parts[0])
	}

	// Generate from IP
	parts := strings.Split(ipv4, ".")
	if len(parts) == 4 {
		return fmt.Sprintf("host-%s", parts[3])
	}

	return fmt.Sprintf("host-%s", strings.ReplaceAll(ipv4, ".", "-"))
}

func isInDHCPRange(ipv4 string, dhcpLow, dhcpHigh int) bool {
	parts := strings.Split(ipv4, ".")
	if len(parts) != 4 {
		return false
	}

	lastOctet, err := strconv.Atoi(parts[3])
	if err != nil {
		return false
	}

	return lastOctet >= dhcpLow && lastOctet <= dhcpHigh
}

func writeConfig(config *Config, filename string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Add header comment
	header := fmt.Sprintf("# Raven Network Monitoring Configuration\n# Generated by raven-discover on %s\n# Contains %d hosts and %d checks\n\n",
		time.Now().Format("2006-01-02 15:04:05"),
		len(config.Hosts),
		len(config.Checks))

	finalData := append([]byte(header), data...)

	if err := os.WriteFile(filename, finalData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
