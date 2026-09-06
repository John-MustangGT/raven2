// internal/monitoring/alert_manager_simple.go - Simplified alert manager for existing engine
package monitoring

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/sirupsen/logrus"
    "raven2/internal/config"
    "raven2/internal/database"
)

// SimpleAlertManager handles alert lifecycle and purging for existing engine
type SimpleAlertManager struct {
    store  database.Store
    config *config.Config
}

// NewSimpleAlertManager creates a new alert manager that works with existing engine
func NewSimpleAlertManager(store database.Store, cfg *config.Config) *SimpleAlertManager {
    return &SimpleAlertManager{
        store:  store,
        config: cfg,
    }
}

// PurgeStaleAlerts removes alerts for hosts/checks that no longer exist in config
func (am *SimpleAlertManager) PurgeStaleAlerts(ctx context.Context) error {
    logrus.Info("Starting alert purge process")
    
    // Get current valid host and check combinations from config
    validCombinations := am.getValidHostCheckCombinations()
    
    // Get all current status entries (these represent active alerts)
    allStatuses, err := am.store.GetStatus(ctx, database.StatusFilters{
        Limit: 10000, // Large limit to get all statuses
    })
    if err != nil {
        return fmt.Errorf("failed to get current statuses: %w", err)
    }
    
    purgedCount := 0
    
    // Check each status entry to see if it's still valid
    for _, status := range allStatuses {
        key := fmt.Sprintf("%s:%s", status.HostID, status.CheckID)
        
        if !validCombinations[key] {
            // This status is for a host/check combination that no longer exists
            logrus.WithFields(logrus.Fields{
                "host_id":  status.HostID,
                "check_id": status.CheckID,
                "status":   status.ExitCode,
            }).Debug("Purging stale alert")

            if err := am.store.DeleteStatus(ctx, status.HostID, status.CheckID); err != nil {
                logrus.WithError(err).WithFields(logrus.Fields{
                    "host_id":  status.HostID,
                    "check_id": status.CheckID,
                }).Error("Failed to purge stale alert")
                continue
            }
            purgedCount++
        }
    }
    
    if purgedCount > 0 {
        logrus.WithField("purged_count", purgedCount).Info("Alert purge completed")
    } else {
        logrus.Debug("No stale alerts found to purge")
    }
    
    return nil
}

// getValidHostCheckCombinations returns a map of valid host:check combinations
func (am *SimpleAlertManager) getValidHostCheckCombinations() map[string]bool {
    valid := make(map[string]bool)
    
    // Build map of valid host IDs
    validHosts := make(map[string]bool)
    for _, host := range am.config.Hosts {
        validHosts[host.ID] = host.Enabled
    }
    
    // Build map of valid host:check combinations
    for _, check := range am.config.Checks {
        if !check.Enabled {
            continue // Skip disabled checks
        }
        
        for _, hostID := range check.Hosts {
            // Only include if host exists and is enabled
            if validHosts[hostID] {
                key := fmt.Sprintf("%s:%s", hostID, check.ID)
                valid[key] = true
            }
        }
    }
    
    logrus.WithField("valid_combinations", len(valid)).Debug("Built valid host:check combinations map")
    
    return valid
}

// PurgeOrphanedHosts removes hosts that no longer exist in the configuration
func (am *SimpleAlertManager) PurgeOrphanedHosts(ctx context.Context) error {
    logrus.Debug("Checking for orphaned hosts in database")
    
    // Get current hosts from config
    configHostIDs := make(map[string]bool)
    for _, host := range am.config.Hosts {
        configHostIDs[host.ID] = true
    }
    
    // Get hosts from database
    dbHosts, err := am.store.GetHosts(ctx, database.HostFilters{})
    if err != nil {
        return fmt.Errorf("failed to get database hosts: %w", err)
    }
    
    purgedCount := 0
    
    // Find orphaned hosts
    for _, dbHost := range dbHosts {
        if !configHostIDs[dbHost.ID] {
            logrus.WithFields(logrus.Fields{
                "host_id":   dbHost.ID,
                "host_name": dbHost.Name,
            }).Info("Purging orphaned host from database")
            
            if err := am.store.DeleteHost(ctx, dbHost.ID); err != nil {
                logrus.WithError(err).WithField("host_id", dbHost.ID).Error("Failed to delete orphaned host")
                continue
            }
            
            purgedCount++
        }
    }
    
    if purgedCount > 0 {
        logrus.WithField("purged_hosts", purgedCount).Info("Orphaned host purge completed")
    }
    
    return nil
}

// PurgeOrphanedChecks removes checks that no longer exist in the configuration
func (am *SimpleAlertManager) PurgeOrphanedChecks(ctx context.Context) error {
    logrus.Debug("Checking for orphaned checks in database")
    
    // Get current checks from config
    configCheckIDs := make(map[string]bool)
    for _, check := range am.config.Checks {
        configCheckIDs[check.ID] = true
    }
    
    // Get checks from database
    dbChecks, err := am.store.GetChecks(ctx)
    if err != nil {
        return fmt.Errorf("failed to get database checks: %w", err)
    }
    
    purgedCount := 0
    
    // Find orphaned checks
    for _, dbCheck := range dbChecks {
        if !configCheckIDs[dbCheck.ID] {
            logrus.WithFields(logrus.Fields{
                "check_id":   dbCheck.ID,
                "check_name": dbCheck.Name,
            }).Info("Purging orphaned check from database")
            
            if err := am.store.DeleteCheck(ctx, dbCheck.ID); err != nil {
                logrus.WithError(err).WithField("check_id", dbCheck.ID).Error("Failed to delete orphaned check")
                continue
            }
            
            purgedCount++
        }
    }
    
    if purgedCount > 0 {
        logrus.WithField("purged_checks", purgedCount).Info("Orphaned check purge completed")
    }
    
    return nil
}

// PurgeAll performs a complete purge of stale data
func (am *SimpleAlertManager) PurgeAll(ctx context.Context) error {
    logrus.Info("Starting complete alert and configuration purge")
    
    var errors []string
    
    // Purge orphaned hosts
    if err := am.PurgeOrphanedHosts(ctx); err != nil {
        errors = append(errors, fmt.Sprintf("host purge failed: %v", err))
    }
    
    // Purge orphaned checks
    if err := am.PurgeOrphanedChecks(ctx); err != nil {
        errors = append(errors, fmt.Sprintf("check purge failed: %v", err))
    }
    
    // Purge stale alerts
    if err := am.PurgeStaleAlerts(ctx); err != nil {
        errors = append(errors, fmt.Sprintf("alert purge failed: %v", err))
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("purge completed with errors: %s", strings.Join(errors, "; "))
    }
    
    logrus.Info("Complete purge finished successfully")
    return nil
}

// SchedulePeriodicPurge sets up automatic purging on a schedule
func (am *SimpleAlertManager) SchedulePeriodicPurge(ctx context.Context, interval time.Duration) {
    // Purge immediately on startup
    go func() {
        if err := am.PurgeAll(ctx); err != nil {
            logrus.WithError(err).Error("Initial purge failed")
        }
    }()
    
    // Schedule periodic purging
    ticker := time.NewTicker(interval)
    go func() {
        defer ticker.Stop()
        
        for {
            select {
            case <-ctx.Done():
                logrus.Debug("Stopping periodic purge scheduler")
                return
            case <-ticker.C:
                logrus.Debug("Running scheduled purge")
                if err := am.PurgeAll(ctx); err != nil {
                    logrus.WithError(err).Error("Scheduled purge failed")
                }
            }
        }
    }()
    
    logrus.WithField("interval", interval).Info("Scheduled periodic alert purging")
}
