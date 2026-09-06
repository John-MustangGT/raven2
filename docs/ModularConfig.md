## Best Practices

### 1. Organize by Purpose and Loading Order
Structure your include files with numeric prefixes to control loading order:

```
config.d/
├── 01-checks.yaml            # Base check definitions (loaded first)
├── 02-infrastructure.yaml    # Core infrastructure hosts  
├── 03-lan-hosts.yaml         # LAN host assignments
├── 04-dmz-hosts.yaml         # DMZ host assignments
├── 05-cloud-hosts.yaml       # Cloud host assignments
├── 10-development.yaml       # Dev environment (higher number = loaded later)
└── 99-overrides.yaml         # Final overrides (loaded last)
```

### 2. Separation of Concerns
- **Check Definitions**: Define all checks with full parameters in one file
- **Host Definitions**: Group hosts by location, environment, or team
- **Host Assignments**: Use partial check definitions to assign hosts to checks
- **Overrides**: Use separate files for environment-specific settings

### 3. Smart Check Organization Pattern
```yaml
# 01-checks.yaml - All check definitions
checks:
  - id: "http-check"
    name: "HTTP Health Check"
    type: "http"
    hosts: []  # Empty initially
    # ... full definition

  - id: "ssh-check"
    name: "SSH Service Check"
    type: "tcp"
    hosts: []  # Empty initially
    # ... full definition

# 02-web-hosts.yaml - Web server hosts
hosts:
  - id: "web-01"
    # ... host definition
checks:
  - id: "http-check"
    hosts: ["web-01"]
  - id: "ssh-check"
    hosts: ["web-01"]

# 03-db-hosts.yaml - Database hosts
hosts:
  - id: "db-01"
    # ... host definition
checks:
  - id: "ssh-check"  # Only SSH for DB servers
    hosts: ["db-01"]
```

### 4. Environment-Based Organization
```
config.d/
├── checks/
│   ├── base-checks.yaml      # Common check definitions
│   └── app-checks.yaml       # Application-specific checks
├── hosts/
│   ├── production.yaml       # Production hosts + check assignments
│   ├── staging.yaml          # Staging hosts + check assignments
│   └── development.yaml      # Development hosts + check assignments
└── overrides/
    ├── prod-settings.yaml    # Production-specific settings
    └── dev-settings.yaml     # Development-specific settings
```

### 5. Team-Based Configuration
Different teams can maintain their own configuration files:
```
config.d/
├── 01-base-checks.yaml       # Shared check definitions
├── 10-infrastructure.yaml    # Infrastructure team
├── 20-database.yaml          # Database team  
├── 30-application.yaml       # Application team
├── 40-security.yaml          # Security team
└── 99-overrides.yaml         # DevOps team overrides
```

## Advanced Examples

### Multi-Environment Check Assignment
```yaml
# checks.yaml - Base definitions
checks:
  - id: "http-check"
    name: "HTTP Health Check"
    type: "http"
    hosts: []
    interval:
      ok: "60s"
      warning: "30s" 
      critical: "15s"
    # ... rest of definition

# production.yaml - Production assignment
checks:
  - id: "http-check"
    hosts:
      - "prod-web-01"
      - "prod-web-02"
      - "prod-api-01"

# staging.yaml - Staging assignment
checks:
  - id: "http-check"
    hosts:
      - "staging-web-01"
      - "staging-api-01"

# development.yaml - Override for dev with relaxed intervals
checks:
  - id: "http-check"
    name: "HTTP Health Check (Dev)"
    type: "http"
    hosts:
      - "dev-web-01"
    interval:
      ok: "300s"      # Less frequent monitoring
      warning: "180s"
      critical: "60s"
    threshold: 5        # More tolerant
    # ... rest overridden
```

### Service-Based Organization
```yaml
# mail-service.yaml
hosts:
  - id: "mail-01"
    # ... mail server definition

checks:
  - id: "smtp-check"
    hosts: ["mail-01"]
  - id: "imap-check"
    hosts: ["mail-01"]
  - id: "ssh-check"
    hosts: ["mail-01"]

# web-service.yaml
hosts:
  - id: "web-01"
  - id: "web-02"
    # ... web server definitions

checks:
  - id: "http-check"
    hosts: ["web-01", "web-02"]
  - id: "ssl-check"
    hosts: ["web-01", "web-02"]
  - id: "ssh-check"
    hosts: ["web-01", "web-02"]
```

### Conditional Host Assignment
```yaml
# base-hosts.yaml
hosts:
  - id: "server-01"
    enabled: true
    # ... definition
  - id: "server-02"
    enabled: false  # Maintenance mode
    # ... definition

# monitoring-assignments.yaml
checks:
  - id: "ping-check"
    hosts:
      - "server-01"
      - "server-02"  # Will be monitored even if disabled
```

## Error Handling and Validation

### Smart Merging Validation
The system validates:
- **Unique Host IDs**: Across all include files
- **Valid Check References**: Partial check definitions must reference existing check IDs
- **Host Existence**: Check host assignments reference valid host IDs
- **No Conflicting Definitions**: Full check definitions with the same ID are not allowed

### Common Error Scenarios

1. **Orphaned Check Assignment**:
   ```yaml
   # Error: Assigning hosts to non-existent check
   checks:
     - id: "nonexistent-check"
       hosts: ["some-host"]
   ```
   **Solution**: Ensure the base check definition exists in an earlier-loaded file

2. **Missing Host Reference**:
   ```yaml
   # Error: Referencing undefined host
   checks:
     - id: "existing-check"
       hosts: ["undefined-host"]
   ```
   **Solution**: Define the host or remove the reference

3. **Conflicting Check Definitions**:
   ```yaml
   # File 1
   checks:
     - id: "http-check"
       name: "HTTP Check"
       # ... full definition

   # File 2 (loaded later) - ERROR
   checks:
     - id: "http-check"
       name: "Different HTTP Check"
       # ... different full definition
   ```
   **Solution**: Use partial definitions for host assignments, or ensure only one full definition exists

## Migration Strategies

### From Monolithic to Modular
1. **Phase 1**: Enable includes and move hosts by environment
   ```yaml
   include:
     enabled: true
     directory: "config.d"
   ```

2. **Phase 2**: Extract check definitions to base file
   ```bash
   # Create base checks file
   echo "checks:" > config.d/01-checks.yaml
   # Move all check definitions here with hosts: []
   ```

3. **Phase 3**: Convert check assignments to partial definitions
   ```yaml
   # Instead of full definitions in multiple files
   # Use partial definitions for host assignments
   checks:
     - id: "existing-check-id"
       hosts: ["host1", "host2"]
   ```

4. **Phase 4**: Organize by logical groupings
   ```bash
   mkdir -p config.d/{checks,hosts,environments,overrides}
   # Reorganize files into logical structure
   ```

### Gradual Adoption
```yaml
# Start simple - just split hosts
include:
  enabled: true
  directory: "config.d"

# config.d/additional-hosts.yaml
hosts:
  - id: "new-server"
    # ... definition

# Original checks in main config still work
# Gradually migrate to smart check merging
```

## Performance and Best Practices

### File Loading Performance
- Include files are processed once at startup
- Loading order: alphabetical by filename
- Large numbers of files (100+) may impact startup time
- Consider consolidating very small files

### Memory Considerations
- Host lists are deduplicated automatically
- Large host lists in multiple partial definitions are efficiently merged
- Final configuration size should be reasonable for your deployment

### Configuration Validation
```bash
# Validate configuration before applying
./raven -config config.yaml -validate

# Dry run to see final merged configuration
./raven -config config.yaml -dump-config
```

### Security Considerations
- Include directory should have appropriate permissions (750 or 755)
- Protect sensitive information in include files
- Consider using different include directories for different security zones
- Pattern matching prevents path traversal attacks

This smart check merging system provides maximum flexibility while maintaining clear separation between check definitions and host assignments, making large-scale monitoring configurations much more maintainable.# Modular Configuration with Includes

Raven now supports modular configuration through the `include` feature, allowing you to split your configuration across multiple files for better organization and maintainability.

## Configuration Structure

```
config/
├── config.yaml                 # Main configuration file
└── config.d/                   # Include directory
    ├── database-servers.yaml   # Database host definitions
    ├── web-servers.yaml        # Web server host definitions
    ├── development.yaml        # Development environment
    ├── monitoring-overrides.yaml # Override monitoring settings
    └── custom-checks.yaml      # Application-specific checks
```

## Enabling Includes

Add the following section to your main `config.yaml`:

```yaml
include:
  enabled: true
  directory: "config.d"     # Directory containing include files
  pattern: "*.yaml"         # File pattern (optional, defaults to "*.yaml")
```

### Include Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Whether to enable include processing |
| `directory` | string | - | Directory containing config files to include |
| `pattern` | string | "*.yaml" | Glob pattern for matching files (also matches *.yml) |

## How Includes Work

1. **File Discovery**: Raven scans the include directory for files matching the pattern
2. **Sorted Loading**: Files are loaded in alphabetical order by filename
3. **Smart Merging Strategy**:
   - **Hosts**: Always appended to existing lists
   - **Checks**: Smart merging with partial definition support
   - **Other Sections**: Override main config values when present
4. **Validation**: Final merged configuration is validated for consistency

## Smart Check Merging

Raven supports intelligent check merging that allows you to separate check definitions from host assignments:

### Full Check Definitions
Define complete checks with all parameters:

```yaml
# config.d/checks.yaml - Base check definitions
checks:
  - id: "port-25-check"
    name: "SMTP Service (Port 25)"
    type: "nagios"
    hosts: []  # Empty initially
    interval:
      critical: "2m"
      ok: "15m"
      warning: "5m"
    threshold: 2
    timeout: "10s"
    enabled: true
    options:
      program: "/usr/lib/nagios/plugins/check_smtp"
```

### Partial Check Definitions (Host Assignment)
Assign hosts to existing checks using minimal syntax:

```yaml
# config.d/lan.yaml - Assign checks to LAN hosts
checks:
  - id: "port-25-check"
    hosts:
      - "kara"
      - "elektra"
      - "hawkgirl"

# config.d/dmz.yaml - Assign same check to DMZ hosts  
checks:
  - id: "port-25-check"
    hosts:
      - "pihole-household"
      - "prometheus"
      - "nebula"
```

### Result
The final merged configuration will have:
```yaml
checks:
  - id: "port-25-check"
    name: "SMTP Service (Port 25)"
    type: "nagios"
    hosts: ["kara", "elektra", "hawkgirl", "pihole-household", "prometheus", "nebula"]
    # ... all other parameters from the full definition
```

### Merging Rules for Checks

1. **Partial Definition Detection**: A check is considered partial if it only has `id` and `hosts` fields
2. **Host Appending**: Hosts from partial definitions are appended to existing check definitions
3. **Duplicate Prevention**: Duplicate hosts in the same check are automatically removed
4. **Full Override**: If a check definition includes any field besides `id` and `hosts`, it completely replaces any existing check with the same ID

## Merging Behavior

### Additive Sections
These sections are **appended** from include files:
- `hosts[]` - All hosts from include files are added to the main config
- `checks[].hosts[]` - Hosts are appended to matching check definitions (smart merging)
- `web.files[]` - Additional files are appended to the list

### Override Sections
These sections **replace** main config values when present in includes:
- `server` - Server configuration settings
- `web` - Web configuration (except files array)
- `database` - Database configuration
- `prometheus` - Prometheus settings
- `monitoring` - Monitoring configuration
- `logging` - Logging settings

## Best Practices

### 1. Organize by Purpose
Structure your include files logically:

```
config.d/
├── 01-infrastructure.yaml    # Core infrastructure hosts
├── 02-applications.yaml      # Application-specific hosts
├── 03-monitoring.yaml        # Monitoring and alerting configs
├── 10-development.yaml       # Dev environment (higher number = loaded later)
└── 99-overrides.yaml         # Final overrides (loaded last)
```

### 2. Use Descriptive Filenames
Choose names that clearly indicate the file's purpose:
- `database-servers.yaml` - Database host configurations
- `web-tier.yaml` - Web application servers
- `network-devices.yaml` - Switches, routers, firewalls
- `custom-checks.yaml` - Application-specific monitoring

### 3. Environment Separation
Create separate files for different environments:
```
config.d/
├── production-hosts.yaml
├── staging-hosts.yaml
├── development-hosts.yaml
└── testing-checks.yaml
```

### 4. Avoid Conflicts
- Use unique host IDs across all files
- Use unique check IDs across all files
- Be mindful of override behavior for non-additive sections

## Example Use Cases

### 1. Team-Based Configuration
Different teams can maintain their own config files:
```
config.d/
├── database-team.yaml      # DBA team maintains database configs
├── web-team.yaml          # Web team maintains web server configs
├── security-team.yaml     # Security team maintains security checks
└── devops-overrides.yaml  # DevOps team maintains global settings
```

### 2. Environment-Specific Settings
```yaml
# config.d/production.yaml
monitoring:
  default_interval: 30s  # More frequent monitoring in production
  max_retries: 5

logging:
  level: "warn"          # Less verbose logging in production

# config.d/development.yaml
monitoring:
  default_interval: 120s # Less frequent in development
  max_retries: 2

logging:
  level: "debug"         # More verbose logging for debugging
```

### 3. Service-Based Organization
```
config.d/
├── web-services/
│   ├── frontend.yaml
│   ├── api-gateway.yaml
│   └── load-balancers.yaml
├── data-services/
│   ├── databases.yaml
│   ├── cache-servers.yaml
│   └── message-queues.yaml
└── infrastructure/
    ├── network-devices.yaml
    └── storage-systems.yaml
```

## Include File Examples

### Basic Host Definition
```yaml
# config.d/web-servers.yaml
hosts:
  - id: "web-01"
    name: "Primary Web Server"
    display_name: "Production Web Server 1"
    ipv4: "10.0.1.10"
    hostname: "web01.prod.company.com"
    group: "web"
    enabled: true
    tags:
      environment: "production"
      role: "webserver"
      datacenter: "us-east-1"
```

### Environment-Specific Overrides
```yaml
# config.d/production-overrides.yaml
monitoring:
  default_interval: 30s
  max_retries: 5
  timeout: 10s

logging:
  level: "info"
  format: "json"

prometheus:
  enabled: true
```

### Application-Specific Checks
```yaml
# config.d/application-checks.yaml
checks:
  - id: "app-health"
    name: "Application Health Check"
    type: "http"
    hosts: ["web-01", "web-02"]
    interval:
      ok: "60s"
      warning: "30s"
      critical: "15s"
    threshold: 2
    timeout: "5s"
    enabled: true
    options:
      scheme: "http"
      path: "/api/health"
      method: "GET"
      expected_status: 200
```

## Error Handling

### Common Issues and Solutions

1. **Duplicate IDs**: Each host and check must have a unique ID across all files
   ```
   Error: duplicate host ID: web-01
   Solution: Ensure host IDs are unique across all config files
   ```

2. **Missing Include Directory**: The specified include directory doesn't exist
   ```
   Error: include directory does not exist: config.d
   Solution: Create the directory or update the path in config.yaml
   ```

3. **Invalid YAML**: Syntax errors in include files
   ```
   Error: failed to parse include file YAML
   Solution: Validate YAML syntax in include files
   ```

4. **Path Traversal**: Security check prevents accessing files outside include directory
   ```
   Error: include.pattern contains invalid glob pattern
   Solution: Use simple glob patterns like "*.yaml" without path separators
   ```

## Migration from Monolithic Config

To migrate from a single large config file:

1. **Create Include Directory**:
   ```bash
   mkdir config.d
   ```

2. **Enable Includes**:
   ```yaml
   # Add to main config.yaml
   include:
     enabled: true
     directory: "config.d"
   ```

3. **Split Configuration**:
   - Move hosts to topic-specific files (e.g., `database-hosts.yaml`)
   - Move checks to logical groups (e.g., `network-checks.yaml`)
   - Keep core settings in main config

4. **Test Configuration**:
   ```bash
   # Validate the merged configuration
   ./raven -config config.yaml -validate
   ```

## Performance Considerations

- Include files are loaded once at startup
- File processing order is alphabetical by filename
- Large numbers of include files (100+) may impact startup time
- Consider consolidating very small files for better performance

## Security Notes

- Include directory should have appropriate file permissions
- Pattern matching is restricted to prevent path traversal attacks
- Only files matching the specified pattern are processed
- Relative paths for include directory are resolved relative to main config file
