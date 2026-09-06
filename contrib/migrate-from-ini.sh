#!/bin/bash
# scripts/migrate-from-ini.sh - Complete migration script from old INI-based system

set -e

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default values
OLD_CONFIG_DIR="${1:-./etc}"
NEW_CONFIG_FILE="${2:-./config.yaml}"
DATA_DIR="${3:-./data}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Check dependencies
check_dependencies() {
    local missing_deps=()
    
    if ! command -v python3 &> /dev/null; then
        missing_deps+=("python3")
    fi
    
    if ! python3 -c "import yaml" &> /dev/null; then
        missing_deps+=("python3-yaml")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        error "Missing dependencies: ${missing_deps[*]}"
        info "Install them with:"
        echo "  Ubuntu/Debian: sudo apt-get install python3 python3-yaml"
        echo "  CentOS/RHEL:   sudo yum install python3 python3-PyYAML"
        echo "  macOS:         brew install python3 && pip3 install PyYAML"
        exit 1
    fi
}

# Create backup of existing configuration
backup_old_config() {
    if [ -d "$OLD_CONFIG_DIR" ]; then
        local backup_file="${DATA_DIR}/old-config-backup-$(date +%Y%m%d-%H%M%S).tar.gz"
        log "Backing up old configuration to: $backup_file"
        mkdir -p "$DATA_DIR"
        tar -czf "$backup_file" "$OLD_CONFIG_DIR" 2>/dev/null || warn "Some files could not be backed up"
        echo "   Backup created: $backup_file"
    else
        warn "Old configuration directory '$OLD_CONFIG_DIR' not found"
    fi
}

# Generate base YAML configuration
generate_base_config() {
    log "Generating base YAML configuration..."
    mkdir -p "$(dirname "$NEW_CONFIG_FILE")"
    
    cat > "$NEW_CONFIG_FILE" << 'EOF'
# Raven Network Monitoring Configuration
# Generated from INI migration
server:
  port: ":8000"
  workers: 3
  read_timeout: "30s"
  write_timeout: "30s"

database:
  type: "boltdb"
  path: "./data/raven.db"
  backup_interval: "1h"
  cleanup_interval: "24h" 
  history_retention: "30d"
  compact_interval: "7d"

prometheus:
  enabled: true
  metrics_path: "/metrics"
  push_gateway: ""

monitoring:
  default_interval: "5m"
  max_retries: 3
  timeout: "30s"
  batch_size: 100

logging:
  level: "info"
  format: "text"

# Hosts and checks will be populated below
hosts: []
checks: []
EOF
    
    log "Base configuration created at: $NEW_CONFIG_FILE"
}

# Convert INI to YAML using Python
convert_ini_to_yaml() {
    local ini_file="$1"
    local yaml_file="$2"
    
    if [ ! -f "$ini_file" ]; then
        warn "INI file '$ini_file' not found, skipping conversion"
        return 0
    fi
    
    log "Converting INI configuration: $ini_file"
    
    python3 << EOF
import configparser
import yaml
import sys
import os
import re
from collections import defaultdict

def clean_id(name):
    """Convert name to valid ID"""
    return re.sub(r'[^a-zA-Z0-9-_]', '-', name.lower().strip())

def parse_interval_string(interval_str):
    """Parse interval string like '90s 1m 30s 30s'"""
    if not interval_str:
        return {
            'ok': '5m',
            'warning': '1m', 
            'critical': '30s',
            'unknown': '30s'
        }
    
    parts = interval_str.strip().split()
    if len(parts) >= 4:
        return {
            'ok': parts[0],
            'warning': parts[1],
            'critical': parts[2], 
            'unknown': parts[3]
        }
    elif len(parts) == 1:
        # Single interval for all states
        return {
            'ok': parts[0],
            'warning': parts[0],
            'critical': parts[0],
            'unknown': parts[0]
        }
    else:
        # Fallback defaults
        return {
            'ok': '5m',
            'warning': '1m',
            'critical': '30s', 
            'unknown': '30s'
        }

def str_to_bool(value):
    """Convert string to boolean"""
    if isinstance(value, bool):
        return value
    return str(value).lower() in ('true', 'yes', '1', 'on', 'enabled')

def parse_hosts_list(hosts_str):
    """Parse space-separated hosts list"""
    if not hosts_str:
        return []
    return [h.strip() for h in hosts_str.split() if h.strip()]

try:
    ini_file = "$ini_file"
    yaml_file = "$yaml_file"
    
    # Read INI file
    config = configparser.ConfigParser()
    config.read(ini_file)
    
    # Load existing YAML to preserve structure
    with open(yaml_file, 'r') as f:
        yaml_config = yaml.safe_load(f)
    
    hosts = []
    checks = []
    converted_sections = 0
    
    print(f"Processing {len(config.sections())} sections...")
    
    for section_name in config.sections():
        section = dict(config[section_name])
        converted_sections += 1
        
        # Detect if this is a host section
        if 'hostname' in section or 'ipv4' in section:
            # This is a host configuration
            host_id = clean_id(section_name)
            host = {
                'id': host_id,
                'name': section_name,
                'display_name': section.get('displayname', section_name),
                'group': section.get('group', 'default'),
                'enabled': str_to_bool(section.get('enabled', 'true')),
                'tags': {}
            }
            
            # Add network information
            if 'hostname' in section and section['hostname'].strip():
                host['hostname'] = section['hostname'].strip()
            if 'ipv4' in section and section['ipv4'].strip():
                host['ipv4'] = section['ipv4'].strip()
                
            # Add tags from other fields
            for key, value in section.items():
                if key not in ['hostname', 'ipv4', 'group', 'enabled', 'displayname'] and value:
                    host['tags'][key] = value
                    
            hosts.append(host)
            print(f"  Host: {section_name} -> {host_id}")
            
        elif 'checkwith' in section or section_name.lower().startswith('check'):
            # This is a check configuration
            check_id = clean_id(section_name)
            check = {
                'id': check_id,
                'name': section_name,
                'type': section.get('checkwith', 'ping'),
                'enabled': str_to_bool(section.get('enabled', 'true')),
                'threshold': int(section.get('threshold', '3')),
                'timeout': section.get('timeout', '30s'),
                'hosts': parse_hosts_list(section.get('hosts', '')),
                'interval': parse_interval_string(section.get('interval', '')),
                'options': {}
            }
            
            # Add check-specific options
            reserved_keys = ['checkwith', 'enabled', 'threshold', 'timeout', 'hosts', 'interval']
            for key, value in section.items():
                if key not in reserved_keys and value:
                    check['options'][key] = value
                    
            checks.append(check)
            print(f"  Check: {section_name} -> {check_id} ({check['type']})")
        
        else:
            print(f"  Unknown section type: {section_name} (skipping)")
    
    # Update YAML configuration
    yaml_config['hosts'] = hosts
    yaml_config['checks'] = checks
    
    # Write updated YAML
    with open(yaml_file, 'w') as f:
        yaml.dump(yaml_config, f, default_flow_style=False, indent=2, sort_keys=False)
    
    print(f"\\nConversion completed successfully!")
    print(f"  Hosts converted: {len(hosts)}")
    print(f"  Checks converted: {len(checks)}")
    print(f"  Total sections processed: {converted_sections}")
    
except Exception as e:
    print(f"Error during conversion: {e}", file=sys.stderr)
    sys.exit(1)
EOF

    if [ $? -eq 0 ]; then
        log "INI to YAML conversion completed successfully"
    else
        error "Failed to convert INI to YAML"
        exit 1
    fi
}

# Validate the generated YAML
validate_yaml() {
    log "Validating generated YAML configuration..."
    
    python3 << EOF
import yaml
import sys

try:
    with open('$NEW_CONFIG_FILE', 'r') as f:
        config = yaml.safe_load(f)
    
    # Basic validation
    required_sections = ['server', 'database', 'monitoring', 'hosts', 'checks']
    missing_sections = []
    
    for section in required_sections:
        if section not in config:
            missing_sections.append(section)
    
    if missing_sections:
        print(f"Missing required sections: {missing_sections}", file=sys.stderr)
        sys.exit(1)
    
    print(f"YAML validation successful")
    print(f"  Hosts: {len(config.get('hosts', []))}")
    print(f"  Checks: {len(config.get('checks', []))}")
    
except yaml.YAMLError as e:
    print(f"YAML syntax error: {e}", file=sys.stderr)
    sys.exit(1)
except Exception as e:
    print(f"Validation error: {e}", file=sys.stderr)
    sys.exit(1)
EOF

    if [ $? -eq 0 ]; then
        log "YAML validation passed"
    else
        error "YAML validation failed"
        exit 1
    fi
}

# Generate migration report
generate_report() {
    log "Generating migration report..."
    
    local report_file="${DATA_DIR}/migration-report-$(date +%Y%m%d-%H%M%S).txt"
    
    cat > "$report_file" << EOF
Raven Migration Report
Generated: $(date)

Migration Details:
  Source:      $OLD_CONFIG_DIR
  Target:      $NEW_CONFIG_FILE
  Data Dir:    $DATA_DIR

Files Processed:
EOF

    # List INI files that were processed
    if [ -d "$OLD_CONFIG_DIR" ]; then
        find "$OLD_CONFIG_DIR" -name "*.ini" -type f >> "$report_file"
    fi
    
    cat >> "$report_file" << EOF

Configuration Summary:
EOF
    
    # Add YAML summary using Python
    python3 << EOF >> "$report_file"
import yaml

try:
    with open('$NEW_CONFIG_FILE', 'r') as f:
        config = yaml.safe_load(f)
    
    print(f"  Server Port: {config['server']['port']}")
    print(f"  Workers: {config['server']['workers']}")
    print(f"  Database: {config['database']['type']} ({config['database']['path']})")
    print(f"  Prometheus: {'Enabled' if config['prometheus']['enabled'] else 'Disabled'}")
    print(f"  Hosts: {len(config.get('hosts', []))}")
    print(f"  Checks: {len(config.get('checks', []))}")
    
    if config.get('hosts'):
        print(f"\\n  Host Groups:")
        groups = {}
        for host in config['hosts']:
            group = host.get('group', 'default')
            groups[group] = groups.get(group, 0) + 1
        for group, count in groups.items():
            print(f"    {group}: {count} hosts")
    
    if config.get('checks'):
        print(f"\\n  Check Types:")
        types = {}
        for check in config['checks']:
            check_type = check.get('type', 'unknown')
            types[check_type] = types.get(check_type, 0) + 1
        for check_type, count in types.items():
            print(f"    {check_type}: {count} checks")
            
except Exception as e:
    print(f"  Error reading config: {e}")
EOF

    cat >> "$report_file" << EOF

Next Steps:
  1. Review the generated configuration: $NEW_CONFIG_FILE
  2. Test the new configuration: ./raven -config $NEW_CONFIG_FILE
  3. Start the service: systemctl start raven
  4. Access web interface: http://localhost:8000

Backup Location:
  Old config backup available in: $DATA_DIR/
EOF

    log "Migration report saved to: $report_file"
}

# Show usage information
show_usage() {
    cat << EOF
Usage: $0 [OLD_CONFIG_DIR] [NEW_CONFIG_FILE] [DATA_DIR]

Migrates Raven monitoring configuration from INI format to YAML.

Arguments:
  OLD_CONFIG_DIR    Directory containing INI config files (default: ./etc)
  NEW_CONFIG_FILE   Path for new YAML config file (default: ./config.yaml)
  DATA_DIR          Directory for data and backups (default: ./data)

Examples:
  $0                                    # Use defaults
  $0 /etc/raven config.yaml ./data     # Specify all paths
  $0 ./old-config                      # Only specify source directory

The script will:
  1. Backup existing configuration
  2. Generate base YAML configuration
  3. Convert INI files to YAML format
  4. Validate the generated configuration
  5. Generate a migration report

Requirements:
  - python3
  - python3-yaml (PyYAML)
EOF
}

# Main function
main() {
    # Check for help flag
    if [[ "$1" == "-h" || "$1" == "--help" ]]; then
        show_usage
        exit 0
    fi
    
    echo -e "${BLUE}🔄 Raven INI to YAML Migration Tool${NC}"
    echo "======================================"
    
    # Check dependencies
    check_dependencies
    
    # Create data directory
    mkdir -p "$DATA_DIR"
    
    # Backup old configuration
    backup_old_config
    
    # Generate base YAML configuration
    generate_base_config
    
    # Convert INI files if they exist
    if [ -f "$OLD_CONFIG_DIR/raven.ini" ]; then
        convert_ini_to_yaml "$OLD_CONFIG_DIR/raven.ini" "$NEW_CONFIG_FILE"
    else
        # Look for other INI files
        local ini_files=($(find "$OLD_CONFIG_DIR" -name "*.ini" -type f 2>/dev/null || true))
        if [ ${#ini_files[@]} -gt 0 ]; then
            log "Found ${#ini_files[@]} INI files, processing the first one: ${ini_files[0]}"
            convert_ini_to_yaml "${ini_files[0]}" "$NEW_CONFIG_FILE"
        else
            warn "No INI files found in $OLD_CONFIG_DIR"
            info "Generated base configuration only"
        fi
    fi
    
    # Validate generated YAML
    validate_yaml
    
    # Generate migration report
    generate_report
    
    echo "======================================"
    log "Migration completed successfully! ✅"
    echo ""
    echo "Next steps:"
    echo "  1. Review configuration: $NEW_CONFIG_FILE"
    echo "  2. Test the new system:   ./raven -config $NEW_CONFIG_FILE"
    echo "  3. Access web interface:  http://localhost:8000"
    echo ""
    echo "Configuration summary:"
    echo "  📁 Data directory:       $DATA_DIR"
    echo "  ⚙️  Config file:          $NEW_CONFIG_FILE"
    echo "  📊 Web interface:        http://localhost:8000"
    echo "  📈 Prometheus metrics:   http://localhost:8000/metrics"
}

# Run main function
main "$@"
