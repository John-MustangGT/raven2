// Enhanced js/utils.js - Utility functions with check name support
window.RavenUtils = {
    formatTime(timestamp) {
        if (!timestamp) return 'Never';
        return new Date(timestamp).toLocaleString();
    },

    formatDuration(duration) {
        if (!duration) return 'Just now';
        
        const seconds = Math.floor(duration / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);
        
        if (days > 0) return `${days}d ${hours % 24}h`;
        if (hours > 0) return `${hours}h ${minutes % 60}m`;
        if (minutes > 0) return `${minutes}m`;
        return `${seconds}s`;
    },

    formatBuildTime(buildTime) {
        if (!buildTime || buildTime === 'unknown') return 'Unknown';
        try {
            return new Date(buildTime).toLocaleString();
        } catch {
            return buildTime;
        }
    },

    getStatusName(exitCode) {
        switch (exitCode) {
            case 0: return 'ok';
            case 1: return 'warning';
            case 2: return 'critical';
            default: return 'unknown';
        }
    },

    calculateDuration(timestamp) {
        if (!timestamp) return 0;
        return Date.now() - new Date(timestamp).getTime();
    },

    showNotification(app, type, message) {
        app.notification = { show: true, type, message };
        setTimeout(() => {
            app.notification.show = false;
        }, 5000);
    },

    getEmptyHostForm() {
        return {
            name: '',
            display_name: '',
            ipv4: '',
            hostname: '',
            group: 'default',
            enabled: true
        };
    },

    getEmptyCheckForm() {
        return {
            name: '',
            type: 'ping',
            hosts: [],
            interval: {
                ok: '5m',
                warning: '2m',
                critical: '1m',
                unknown: '1m'
            },
            threshold: 3,
            timeout: '30s',
            enabled: true,
            options: {}
        };
    },

    // Enhanced functions for IP checks and soft fails
    formatIPStatus(ipOK, lastChecked) {
        if (!lastChecked) return 'Never checked';
        
        const status = ipOK ? 'OK' : 'Failed';
        const time = this.formatTime(lastChecked);
        return `${status} (${time})`;
    },

    // ENHANCED: Format soft fail status with check name support
    formatSoftFailStatus(softFailInfo, useCheckNames = true) {
        if (!softFailInfo) return '';
        
        // Handle both old format (single object) and new format (with check_name property)
        if (softFailInfo.current_fails !== undefined) {
            // Single soft fail object
            const { current_fails, threshold_max, first_fail_time, check_name } = softFailInfo;
            const duration = this.calculateDuration(first_fail_time);
            const durationStr = this.formatDuration(duration);
            
            const name = useCheckNames && check_name ? check_name : 'Check';
            return `${name}: ${current_fails}/${threshold_max} fails (${durationStr})`;
        }
        
        // Multiple soft fails (object with check IDs as keys)
        const results = [];
        for (const [checkId, failInfo] of Object.entries(softFailInfo)) {
            const { current_fails, threshold_max, first_fail_time, check_name } = failInfo;
            const duration = this.calculateDuration(first_fail_time);
            const durationStr = this.formatDuration(duration);
            
            const name = useCheckNames && check_name ? check_name : checkId;
            results.push(`${name}: ${current_fails}/${threshold_max} fails (${durationStr})`);
        }
        
        return results.join(', ');
    },

    // ENHANCED: Format OK duration with check name support
    formatOKDuration(okInfo, useCheckNames = true) {
        if (!okInfo) return '';
        
        // Handle both old format (single object) and new format (with check_name property)
        if (okInfo.ok_since !== undefined) {
            // Single OK duration object
            const { ok_since, duration, check_count, check_name } = okInfo;
            const name = useCheckNames && check_name ? check_name : 'Check';
            return `${name}: OK since ${this.formatTime(ok_since)} (${duration}, ${check_count} checks)`;
        }
        
        // Multiple OK durations (object with check IDs as keys)
        const results = [];
        for (const [checkId, okInfo] of Object.entries(okInfo)) {
            const { ok_since, duration, check_count, check_name } = okInfo;
            const name = useCheckNames && check_name ? check_name : checkId;
            results.push(`${name}: OK for ${duration} (${check_count} checks)`);
        }
        
        return results.join(', ');
    },

    getIPCheckClass(ipOK) {
        return ipOK ? 'ip-check-ok' : 'ip-check-fail';
    },

    getSoftFailSeverity(softFailInfo) {
        if (!softFailInfo) return 'unknown';
        
        const { current_fails, threshold_max } = softFailInfo;
        const ratio = current_fails / threshold_max;
        
        if (ratio >= 1) return 'critical';
        if (ratio >= 0.8) return 'warning';
        if (ratio >= 0.5) return 'warning';
        return 'unknown';
    },

    // NEW: Get check name with fallback to check ID
    getCheckNameWithFallback(checkId, checkNamesMapping) {
        if (checkNamesMapping && checkNamesMapping[checkId]) {
            return checkNamesMapping[checkId];
        }
        return checkId;
    },

    // NEW: Format monitoring results for display
    formatMonitoringResultsForDisplay(monitoringData, host, type = 'soft_fail') {
        if (!monitoringData || Object.keys(monitoringData).length === 0) {
            return null;
        }

        const results = [];
        for (const [checkId, info] of Object.entries(monitoringData)) {
            let checkName = checkId; // fallback
            
            // Try multiple ways to get the check name
            if (info.check_name) {
                checkName = info.check_name;
            } else if (host.check_names && host.check_names[checkId]) {
                checkName = host.check_names[checkId];
            }
            
            const result = {
                checkId: checkId,
                checkName: checkName,
                ...info
            };
            
            results.push(result);
        }
        
        // Sort by check name for consistent display
        results.sort((a, b) => a.checkName.localeCompare(b.checkName));
        
        return results;
    },

    // NEW: Get monitoring summary for a host
    getMonitoringSummary(host) {
        const summary = {
            totalChecks: 0,
            healthyChecks: 0,
            failingChecks: 0,
            criticalFailures: 0,
            softFailures: 0,
            checkNames: []
        };

        // Count checks from check_names mapping
        if (host.check_names) {
            summary.totalChecks = Object.keys(host.check_names).length;
            summary.checkNames = Object.values(host.check_names);
        }

        // Count healthy checks
        if (host.ok_duration) {
            summary.healthyChecks = Object.keys(host.ok_duration).length;
        }

        // Count and categorize failures
        if (host.soft_fail_info) {
            summary.failingChecks = Object.keys(host.soft_fail_info).length;
            
            for (const failInfo of Object.values(host.soft_fail_info)) {
                const ratio = failInfo.current_fails / failInfo.threshold_max;
                if (ratio >= 1) {
                    summary.criticalFailures++;
                } else {
                    summary.softFailures++;
                }
            }
        }

        return summary;
    },

    // NEW: Format check type display
    formatCheckTypeDisplay(checkType) {
        const typeMap = {
            'ping': 'Network Ping',
            'http': 'HTTP Check',
            'https': 'HTTPS Check',
            'nagios': 'Nagios Plugin',
            'icinga': 'Icinga Plugin',
            'tcp': 'TCP Connection',
            'dns': 'DNS Lookup',
            'ssh': 'SSH Check'
        };
        
        return typeMap[checkType?.toLowerCase()] || checkType || 'Unknown';
    },

    // Status analysis helpers
    analyzeStatusTrend(statuses) {
        if (!statuses || statuses.length === 0) return 'stable';
        
        const recent = statuses.slice(0, 5);
        const okCount = recent.filter(s => s.exit_code === 0).length;
        const failCount = recent.length - okCount;
        
        if (failCount === 0) return 'stable';
        if (failCount >= 3) return 'declining';
        if (okCount >= 3) return 'improving';
        return 'unstable';
    },

    getStatusTrendIcon(trend) {
        switch (trend) {
            case 'stable': return 'fas fa-check-circle';
            case 'improving': return 'fas fa-arrow-up';
            case 'declining': return 'fas fa-arrow-down';
            case 'unstable': return 'fas fa-exclamation-triangle';
            default: return 'fas fa-question-circle';
        }
    },

    // Enhanced alert metrics calculation
    calculateAlertMetrics(alerts) {
        const metrics = {
            active: 0,
            critical: 0,
            warning: 0,
            unknown: 0,
            soft_fails: 0,
            hard_fails: 0
        };

        alerts.forEach(alert => {
            if (alert.severity !== 'ok') {
                metrics.active++;
                metrics[alert.severity]++;

                if (alert.soft_fails_info) {
                    if (alert.soft_fails_info.current_fails >= alert.soft_fails_info.threshold_max) {
                        metrics.hard_fails++;
                    } else {
                        metrics.soft_fails++;
                    }
                }
            }
        });

        return metrics;
    },

    // Host status aggregation
    aggregateHostStatus(host) {
        if (!host.soft_fail_info && !host.ok_duration) {
            return host.status;
        }

        // If there are soft fails, show the most severe
        if (host.soft_fail_info && Object.keys(host.soft_fail_info).length > 0) {
            let maxSeverity = 'unknown';
            
            Object.values(host.soft_fail_info).forEach(failInfo => {
                const severity = this.getSoftFailSeverity(failInfo);
                if (severity === 'critical') maxSeverity = 'critical';
                else if (severity === 'warning' && maxSeverity !== 'critical') maxSeverity = 'warning';
            });
            
            return maxSeverity;
        }

        // If all checks are OK with duration info, return OK
        if (host.ok_duration && Object.keys(host.ok_duration).length > 0) {
            return 'ok';
        }

        return host.status || 'unknown';
    },

    // Generate mini status history dots
    generateStatusHistory(statuses, limit = 10) {
        if (!statuses || statuses.length === 0) return [];
        
        return statuses.slice(0, limit).map(status => ({
            status: this.getStatusName(status.exit_code),
            timestamp: status.timestamp,
            tooltip: `${this.getStatusName(status.exit_code).toUpperCase()} at ${this.formatTime(status.timestamp)}`
        }));
    },

    // Check if IP address is valid
    isValidIPAddress(ip) {
        if (!ip) return false;
        
        const ipv4Regex = /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
        const ipv6Regex = /^(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$/;
        
        return ipv4Regex.test(ip) || ipv6Regex.test(ip);
    },

    // Format threshold display
    formatThreshold(current, max) {
        if (!current || !max) return '';
        
        const percentage = Math.round((current / max) * 100);
        return `${current}/${max} (${percentage}%)`;
    },

    // Get appropriate icon for check type
    getCheckTypeIcon(checkType) {
        switch (checkType?.toLowerCase()) {
            case 'ping': return 'fas fa-wifi';
            case 'http': return 'fab fa-html5';
            case 'https': return 'fas fa-lock';
            case 'nagios': return 'fas fa-cog';
            case 'icinga': return 'fas fa-cogs';
            case 'tcp': return 'fas fa-network-wired';
            case 'dns': return 'fas fa-globe';
            case 'ssh': return 'fas fa-terminal';
            default: return 'fas fa-question-circle';
        }
    },

    // Sort hosts by priority (failing first, then by name)
    sortHostsByPriority(hosts) {
        return [...hosts].sort((a, b) => {
            const aPriority = this.getHostPriority(a);
            const bPriority = this.getHostPriority(b);
            
            if (aPriority !== bPriority) {
                return bPriority - aPriority; // Higher priority first
            }
            
            // Same priority, sort by name
            const aName = a.display_name || a.name || '';
            const bName = b.display_name || b.name || '';
            return aName.localeCompare(bName);
        });
    },

    // Get host priority for sorting (higher number = higher priority)
    getHostPriority(host) {
        // Critical issues get highest priority
        if (host.status === 'critical') return 4;
        
        // Soft fails get high priority
        if (host.soft_fail_info && Object.keys(host.soft_fail_info).length > 0) {
            const maxRatio = Math.max(...Object.values(host.soft_fail_info).map(info => 
                info.current_fails / info.threshold_max
            ));
            if (maxRatio >= 0.8) return 3;
            if (maxRatio >= 0.5) return 2;
        }
        
        // Warning status gets medium priority
        if (host.status === 'warning') return 2;
        
        // IP connectivity issues
        if (host.ip_address_ok === false) return 1;
        
        // Everything else (OK, unknown) gets lowest priority
        return 0;
    },

    // Enhanced filtering helpers
    filterHostsByStatus(hosts, statusFilter) {
        if (!statusFilter || statusFilter === 'all') return hosts;
        
        return hosts.filter(host => {
            const aggregatedStatus = this.aggregateHostStatus(host);
            
            if (statusFilter === 'failing') {
                return ['critical', 'warning', 'unknown'].includes(aggregatedStatus);
            }
            
            if (statusFilter === 'soft_failing') {
                return host.soft_fail_info && Object.keys(host.soft_fail_info).length > 0;
            }
            
            if (statusFilter === 'ip_issues') {
                return host.ip_address_ok === false;
            }
            
            return aggregatedStatus === statusFilter;
        });
    },

    // Enhanced search functionality WITH check name support
    searchHosts(hosts, query) {
        if (!query) return hosts;
        
        const searchTerm = query.toLowerCase();
        
        return hosts.filter(host => {
            // Basic field matching
            const basicMatch = [
                host.name,
                host.display_name,
                host.ipv4,
                host.hostname,
                host.group
            ].some(field => field?.toLowerCase().includes(searchTerm));
            
            if (basicMatch) return true;
            
            // Status matching
            const statusMatch = host.status?.toLowerCase().includes(searchTerm);
            if (statusMatch) return true;
            
            // Check names matching (NEW)
            if (host.check_names) {
                const checkNameMatch = Object.values(host.check_names).some(checkName =>
                    checkName?.toLowerCase().includes(searchTerm)
                );
                if (checkNameMatch) return true;
            }
            
            // Tag matching (if tags exist)
            if (host.tags) {
                const tagMatch = Object.entries(host.tags).some(([key, value]) =>
                    key.toLowerCase().includes(searchTerm) || 
                    value.toLowerCase().includes(searchTerm)
                );
                if (tagMatch) return true;
            }
            
            return false;
        });
    },

    // Enhanced export functionality with check names
    exportHostsToCSV(hosts) {
        const headers = [
            'Name',
            'Display Name',
            'IP Address',
            'Hostname',
            'Group',
            'Status',
            'IP Check',
            'Last Check',
            'Enabled',
            'Active Checks',
            'Failing Tests',
            'Healthy Tests'
        ];
        
        const rows = hosts.map(host => {
            const summary = this.getMonitoringSummary(host);
            
            return [
                host.name || '',
                host.display_name || '',
                host.ipv4 || '',
                host.hostname || '',
                host.group || '',
                host.status || '',
                host.ip_address_ok ? 'OK' : 'Failed',
                this.formatTime(host.last_check),
                host.enabled ? 'Yes' : 'No',
                summary.checkNames.join('; '),
                summary.failingChecks > 0 ? summary.failingChecks + ' tests' : 'None',
                summary.healthyChecks > 0 ? summary.healthyChecks + ' tests' : 'None'
            ];
        });
        
        return this.arrayToCSV([headers, ...rows]);
    },

    // NEW: Export monitoring results to CSV
    exportMonitoringResultsToCSV(hosts) {
        const headers = [
            'Host Name',
            'Check Name', 
            'Check Type',
            'Status',
            'Failures',
            'Threshold',
            'First Fail Time',
            'OK Since',
            'OK Duration'
        ];
        
        const rows = [];
        
        hosts.forEach(host => {
            const hostName = host.display_name || host.name;
            
            // Add soft fail entries
            if (host.soft_fail_info) {
                Object.values(host.soft_fail_info).forEach(failInfo => {
                    rows.push([
                        hostName,
                        failInfo.check_name || 'Unknown',
                        'Failing',
                        'Soft Fail',
                        `${failInfo.current_fails}/${failInfo.threshold_max}`,
                        failInfo.threshold_max,
                        this.formatTime(failInfo.first_fail_time),
                        '',
                        ''
                    ]);
                });
            }
            
            // Add OK entries
            if (host.ok_duration) {
                Object.values(host.ok_duration).forEach(okInfo => {
                    rows.push([
                        hostName,
                        okInfo.check_name || 'Unknown',
                        'Healthy',
                        'OK',
                        '',
                        '',
                        '',
                        this.formatTime(okInfo.ok_since),
                        okInfo.duration
                    ]);
                });
            }
        });
        
        return this.arrayToCSV([headers, ...rows]);
    },

    exportAlertsToCSV(alerts) {
        const headers = [
            'Timestamp',
            'Severity',
            'Host',
            'Check Name',
            'Message',
            'Duration',
            'Soft Fails',
            'OK Since'
        ];
        
        const rows = alerts.map(alert => [
            this.formatTime(alert.timestamp),
            alert.severity,
            alert.host_name || alert.host,
            alert.check_name || alert.check || 'Unknown Check',  // Enhanced with check name
            alert.message || '',
            this.formatDuration(alert.duration),
            alert.soft_fails_info ? this.formatSoftFailStatus(alert.soft_fails_info) : '',
            alert.ok_info ? this.formatTime(alert.ok_info.ok_since) : ''
        ]);
        
        return this.arrayToCSV([headers, ...rows]);
    },

    // CSV helper function
    arrayToCSV(data) {
        return data.map(row => 
            row.map(field => {
                const escaped = String(field).replace(/"/g, '""');
                return escaped.includes(',') || escaped.includes('"') || escaped.includes('\n') 
                    ? `"${escaped}"` 
                    : escaped;
            }).join(',')
        ).join('\n');
    },

    // Download helper
    downloadCSV(csvContent, filename) {
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const link = document.createElement('a');
        
        if (link.download !== undefined) {
            const url = URL.createObjectURL(blob);
            link.setAttribute('href', url);
            link.setAttribute('download', filename);
            link.style.visibility = 'hidden';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        }
    },

    // Local storage helpers for user preferences
    saveUserPreference(key, value) {
        try {
            localStorage.setItem(`raven_${key}`, JSON.stringify(value));
        } catch (error) {
            console.warn('Failed to save user preference:', error);
        }
    },

    getUserPreference(key, defaultValue = null) {
        try {
            const item = localStorage.getItem(`raven_${key}`);
            return item ? JSON.parse(item) : defaultValue;
        } catch (error) {
            console.warn('Failed to load user preference:', error);
            return defaultValue;
        }
    },

    // Color coding helpers for status displays
    getStatusColor(status, opacity = 1) {
        const colors = {
            ok: `rgba(16, 185, 129, ${opacity})`,      // green
            warning: `rgba(245, 158, 11, ${opacity})`, // yellow
            critical: `rgba(239, 68, 68, ${opacity})`, // red
            unknown: `rgba(156, 163, 175, ${opacity})` // gray
        };
        
        return colors[status] || colors.unknown;
    },

    // Generate status badge HTML
    generateStatusBadge(status, additionalInfo = '') {
        const statusClass = `status-${status}`;
        const additionalContent = additionalInfo ? `<small>${additionalInfo}</small>` : '';
        
        return `
            <span class="status-badge ${statusClass}">
                <div class="status-indicator ${statusClass}"></div>
                ${status.toUpperCase()}
                ${additionalContent}
            </span>
        `;
    },

    // Debounce function for search inputs
    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    },

    // Throttle function for scroll events
    throttle(func, limit) {
        let inThrottle;
        return function() {
            const args = arguments;
            const context = this;
            if (!inThrottle) {
                func.apply(context, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    },

    // Format numbers with appropriate suffixes
    formatNumber(num) {
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        } else if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        }
        return num.toString();
    },

    // Generate unique ID for components
    generateId(prefix = 'raven') {
        return `${prefix}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    },

    // Validate configuration objects
    validateHostConfig(hostConfig) {
        const errors = [];
        
        if (!hostConfig.name || hostConfig.name.trim() === '') {
            errors.push('Host name is required');
        }
        
        if (hostConfig.ipv4 && !this.isValidIPAddress(hostConfig.ipv4)) {
            errors.push('Invalid IP address format');
        }
        
        if (hostConfig.name && !/^[a-zA-Z0-9_-]+$/.test(hostConfig.name)) {
            errors.push('Host name can only contain letters, numbers, underscores, and dashes');
        }
        
        return {
            isValid: errors.length === 0,
            errors
        };
    },

    validateCheckConfig(checkConfig) {
        const errors = [];
        
        if (!checkConfig.name || checkConfig.name.trim() === '') {
            errors.push('Check name is required');
        }
        
        if (!checkConfig.type || checkConfig.type.trim() === '') {
            errors.push('Check type is required');
        }
        
        if (!checkConfig.hosts || checkConfig.hosts.length === 0) {
            errors.push('At least one host must be selected');
        }
        
        if (checkConfig.threshold && (checkConfig.threshold < 1 || checkConfig.threshold > 10)) {
            errors.push('Threshold must be between 1 and 10');
        }
        
        // Validate interval formats
        if (checkConfig.interval) {
            Object.entries(checkConfig.interval).forEach(([state, interval]) => {
                if (interval && !this.isValidDuration(interval)) {
                    errors.push(`Invalid interval format for ${state}: ${interval}`);
                }
            });
        }
        
        return {
            isValid: errors.length === 0,
            errors
        };
    },

    // Validate duration strings (e.g., "5m", "30s", "1h")
    isValidDuration(duration) {
        if (!duration) return true; // Empty is valid
        return /^\d+[smhd]$/.test(duration.trim());
    },

    // Get human-readable error messages
    getErrorMessage(error) {
        if (error.response && error.response.data && error.response.data.error) {
            return error.response.data.error;
        }
        
        if (error.message) {
            return error.message;
        }
        
        return 'An unknown error occurred';
    }
};
