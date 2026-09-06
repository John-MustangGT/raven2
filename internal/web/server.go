// internal/web/server.go
package web

import (
    "context"
    "net/http"
    "path/filepath"
    "sync"
    "time"
    "os"
    "strings"
    "fmt"
    "mime"

    "github.com/gin-gonic/gin"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/sirupsen/logrus"
    "raven2/internal/config"
    "raven2/internal/database"
    "raven2/internal/metrics"
    "raven2/internal/monitoring"
)

type Server struct {
    config    *config.Config
    store     database.Store
    engine    *monitoring.Engine
    metrics   *metrics.Collector
    router    *gin.Engine
    wsMu      sync.Mutex
    wsClients map[*WSClient]bool
    server    *http.Server
}

func NewServer(cfg *config.Config, store database.Store, engine *monitoring.Engine, metricsCollector *metrics.Collector) *Server {
    if cfg.Logging.Level != "debug" {
        gin.SetMode(gin.ReleaseMode)
    }

    router := gin.New()
    router.Use(gin.LoggerWithFormatter(accessLogFormatter(underSystemdJournal())))
    router.Use(gin.Recovery())
    router.Use(corsMiddleware())

    server := &Server{
        config:    cfg,
        store:     store,
        engine:    engine,
        metrics:   metricsCollector,
        router:    router,
        wsClients: make(map[*WSClient]bool),
    }

    server.setupRoutes()
    return server
}

// underSystemdJournal reports whether our stdout/stderr are being captured
// directly by the systemd journal. systemd sets JOURNAL_STREAM on a unit's
// environment in exactly that case (see systemd.exec(5)), and journald
// already stamps every line it receives, so our own timestamp would just
// be a redundant, noisier duplicate of the one journalctl already shows.
func underSystemdJournal() bool {
    return os.Getenv("JOURNAL_STREAM") != ""
}

// accessLogFormatter is gin's default access log line, minus the leading
// timestamp when journald is already going to add one.
func accessLogFormatter(underJournal bool) gin.LogFormatter {
    return func(p gin.LogFormatterParams) string {
        if p.Latency > time.Minute {
            p.Latency = p.Latency.Truncate(time.Second)
        }

        var prefix string
        if !underJournal {
            prefix = p.TimeStamp.Format("2006/01/02 - 15:04:05") + " | "
        }

        var statusColor, methodColor, resetColor string
        if p.IsOutputColor() {
            statusColor = p.StatusCodeColor()
            methodColor = p.MethodColor()
            resetColor = p.ResetColor()
        }

        return fmt.Sprintf("[GIN] %s%s %3d %s| %13v | %15s | %s%-7s%s %s\n",
            prefix,
            statusColor, p.StatusCode, resetColor,
            p.Latency,
            p.ClientIP,
            methodColor, p.Method, resetColor,
            p.Path,
        )
    }
}

func (s *Server) Start(ctx context.Context) error {
    s.server = &http.Server{
        Addr:         s.config.Server.Port,
        Handler:      s.router,
        ReadTimeout:  s.config.Server.ReadTimeout,
        WriteTimeout: s.config.Server.WriteTimeout,
    }

    logrus.WithField("port", s.config.Server.Port).Info("Starting web server")

    // Start metrics update routine
    go s.updateMetricsRoutine(ctx)

    // Start server in goroutine
    go func() {
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logrus.WithError(err).Fatal("Failed to start server")
        }
    }()

    return nil
}

func (s *Server) Stop(ctx context.Context) error {
    if s.server != nil {
        return s.server.Shutdown(ctx)
    }
    return nil
}

func (s *Server) setupRoutes() {
    // Configure static file serving based on config
    if s.config.Web.ServeStatic {
        var staticDir string
        
        // Determine static directory
        if s.config.Web.AssetsDir != "" {
            // Use configured assets directory
            if filepath.IsAbs(s.config.Web.StaticDir) {
                staticDir = s.config.Web.StaticDir
            } else {
                staticDir = filepath.Join(s.config.Web.AssetsDir, s.config.Web.StaticDir)
            }
        } else {
            // Auto-detect static directory
            possibleStaticDirs := []string{
                "./web/static",
                "/usr/lib/raven/web/static",
                "/opt/raven/web/static",
            }
            
            for _, dir := range possibleStaticDirs {
                if _, err := os.Stat(dir); err == nil {
                    staticDir = dir
                    break
                }
            }
        }
        
        // Enable static serving if directory exists
        if staticDir != "" {
            if _, err := os.Stat(staticDir); err == nil {
                s.router.Static("/static", staticDir)
                logrus.WithField("static_dir", staticDir).Debug("Enabled static file serving")
            } else {
                logrus.WithField("static_dir", staticDir).Warn("Configured static directory not found")
            }
        }
    }

    // Setup configurable file routes
    s.setupFileRoutes()

    // API routes
    api := s.router.Group("/api")
    {
        // Host endpoints
        api.GET("/hosts", s.getHosts)
        api.GET("/hosts/:id", s.getHost)
        api.POST("/hosts", s.createHost)
        api.PUT("/hosts/:id", s.updateHost)
        api.DELETE("/hosts/:id", s.deleteHost)

        // Check endpoints
        api.GET("/checks", s.getChecks)
        api.GET("/checks/:id", s.getCheck)
        api.POST("/checks", s.createCheck)
        api.PUT("/checks/:id", s.updateCheck)
        api.DELETE("/checks/:id", s.deleteCheck)

        // Status endpoints
        api.GET("/status", s.getStatus)
        api.GET("/status/history/:host/:check", s.getStatusHistory)

        // Alert endpoints
        api.GET("/alerts", s.getAlerts)
        api.GET("/alerts/summary", s.getAlertsSummary)

        // System endpoints
        api.GET("/stats", s.getStats)
        api.GET("/health", s.healthCheck)
        api.GET("/diagnostics/web", s.webDiagnostics)
        api.GET("/build-info", s.getBuildInfo)

        // web-config endpoints
        api.GET("/web-config", s.getWebConfig)
    }

    // WebSocket endpoint
    s.router.GET("/ws", s.handleWebSocket)

    // Add purge routes
    s.setupPurgeRoutes()

    // Prometheus metrics
    if s.config.Prometheus.Enabled {
        s.router.GET(s.config.Prometheus.MetricsPath, gin.WrapH(promhttp.Handler()))
    }
}

// setupFileRoutes configures routes for files specified in the config
func (s *Server) setupFileRoutes() {
    // Root route (either configured or default to index.html)
    rootFile := s.config.Web.Root
    if rootFile == "" {
        rootFile = "index.html"
    }
    
    // Main page route
    s.router.GET("/", func(c *gin.Context) {
        s.serveConfiguredFile(c, rootFile)
    })

    // If files are specified in config, create routes for each
    if len(s.config.Web.Files) > 0 {
        for _, filename := range s.config.Web.Files {
            // Create a closure to capture the filename
            filename := filename // Important: capture the loop variable
            
            // Create route for this file
            route := "/" + filename
            s.router.GET(route, func(c *gin.Context) {
                s.serveConfiguredFile(c, filename)
            })
            
            logrus.WithFields(logrus.Fields{
                "route": route,
                "file":  filename,
            }).Debug("Registered file route")
        }
    } else {
        // Fallback: register common files if no files specified
        commonFiles := []string{
            "styles.css",
            "favicon.ico", 
            "favicon.svg",
        }
        
        for _, filename := range commonFiles {
            filename := filename // Capture loop variable
            route := "/" + filename
            s.router.GET(route, func(c *gin.Context) {
                s.serveConfiguredFile(c, filename)
            })
        }
        
        logrus.Debug("No files specified in config, registered default common files")
    }
}

// serveConfiguredFile serves a file from the configured assets directory
func (s *Server) serveConfiguredFile(c *gin.Context, filename string) {
    filePath := s.findAssetFile(filename)
    
    if filePath == "" {
        logrus.WithField("filename", filename).Error("Asset file not found")
        s.serveFileNotFoundError(c, filename)
        return
    }
    
    // Log which path we're serving from (debug level)
    logrus.WithFields(logrus.Fields{
        "filename": filename,
        "path":     filePath,
    }).Debug("Serving asset file")
    
    // Set appropriate headers based on file type
    s.setFileHeaders(c, filename)
    
    // Serve the file
    c.File(filePath)
}

// findAssetFile searches for a file in the configured assets directory and fallback locations
func (s *Server) findAssetFile(filename string) string {
    var searchPaths []string
    
    // If assets directory is configured, try that first
    if s.config.Web.AssetsDir != "" {
        configuredPath := filepath.Join(s.config.Web.AssetsDir, filename)
        searchPaths = append(searchPaths, configuredPath)
    }
    
    // Add fallback paths for development and standard package locations
    fallbackPaths := []string{
        filepath.Join("web", filename),          // Development path
        filepath.Join("./web", filename),        // Alternative development path
        filepath.Join("/usr/lib/raven/web", filename), // Production package path
        filepath.Join("/opt/raven/web", filename),      // Alternative production path
    }
    searchPaths = append(searchPaths, fallbackPaths...)
    
    // Find the first existing path
    for _, path := range searchPaths {
        if _, err := os.Stat(path); err == nil {
            return path
        }
    }
    
    return "" // File not found
}

// setFileHeaders sets appropriate HTTP headers based on file type
func (s *Server) setFileHeaders(c *gin.Context, filename string) {
    // Determine content type from file extension
    ext := filepath.Ext(filename)
    contentType := mime.TypeByExtension(ext)
    if contentType == "" {
        // Fallback content types for common web files
        switch ext {
        case ".html":
            contentType = "text/html; charset=utf-8"
        case ".css":
            contentType = "text/css"
        case ".js":
            contentType = "application/javascript"
        case ".svg":
            contentType = "image/svg+xml"
        case ".ico":
            contentType = "image/x-icon"
        case ".png":
            contentType = "image/png"
        case ".jpg", ".jpeg":
            contentType = "image/jpeg"
        default:
            contentType = "application/octet-stream"
        }
    }
    
    c.Header("Content-Type", contentType)
    
    // Set caching headers based on file type
    switch {
    case strings.HasSuffix(filename, ".html"):
        // Don't cache HTML files
        c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
        c.Header("Pragma", "no-cache")
        c.Header("Expires", "0")
    case strings.HasSuffix(filename, ".css") || strings.HasSuffix(filename, ".js"):
        // Cache CSS and JS for 1 hour
        c.Header("Cache-Control", "public, max-age=3600")
    case strings.Contains(filename, "favicon"):
        // Cache favicons for 1 year
        c.Header("Cache-Control", "public, max-age=31536000")
    default:
        // Default cache for other static assets
        c.Header("Cache-Control", "public, max-age=86400") // 24 hours
    }
}

// serveFileNotFoundError serves a helpful error page when a configured file is not found
func (s *Server) serveFileNotFoundError(c *gin.Context, filename string) {
    c.Header("Content-Type", "text/html; charset=utf-8")
    c.String(http.StatusNotFound, `
<!DOCTYPE html>
<html>
<head>
    <title>Raven - File Not Found</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background-color: #f5f5f5; }
        .error-container { 
            background: white; 
            padding: 30px; 
            border-radius: 8px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            max-width: 700px;
        }
        h1 { color: #e74c3c; }
        .config-info { background: #e3f2fd; padding: 15px; border-left: 4px solid #2196f3; margin: 20px 0; }
        .paths { background: #f8f9fa; padding: 15px; border-left: 4px solid #e74c3c; margin: 20px 0; }
        .solution { background: #e8f5e9; padding: 15px; border-left: 4px solid #4caf50; margin: 20px 0; }
        code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; font-family: monospace; }
        ul { margin: 10px 0; padding-left: 20px; }
    </style>
</head>
<body>
    <div class="error-container">
        <h1>🐦 Raven - File Not Found</h1>
        <p>The requested file <code>%s</code> could not be found.</p>
        
        <div class="config-info">
            <strong>Current Configuration:</strong><br>
            Assets directory: <code>%s</code><br>
            Configured files: <code>%v</code><br>
            Root file: <code>%s</code>
        </div>
        
        <div class="paths">
            <strong>Searched locations:</strong>
            <ul>%s</ul>
        </div>
        
        <div class="solution">
            <strong>Solutions:</strong><br><br>
            
            <strong>1. Ensure file exists in assets directory:</strong><br>
            <code>ls -la %s</code><br><br>
            
            <strong>2. Check file permissions:</strong><br>
            <code>chmod 644 %s</code><br><br>
            
            <strong>3. Verify configuration in config.yaml:</strong><br>
            Make sure the file is listed in the <code>web.files</code> section.
        </div>
        
        <p><strong>API Status:</strong> ✅ The REST API is working at <code>/api/</code></p>
        <p><strong>Diagnostics:</strong> Visit <code>/api/diagnostics/web</code> for detailed information</p>
        
        <hr>
        <p><em>Raven v2.0 Network Monitoring</em></p>
    </div>
</body>
</html>`, 
        filename,
        s.config.Web.AssetsDir,
        s.config.Web.Files,
        func() string {
            if s.config.Web.Root != "" {
                return s.config.Web.Root
            }
            return "index.html (default)"
        }(),
        s.generateSearchPathsList(filename),
        s.config.Web.AssetsDir,
        filepath.Join(s.config.Web.AssetsDir, filename),
    )
}

// generateSearchPathsList creates an HTML list of searched paths for error display
func (s *Server) generateSearchPathsList(filename string) string {
    var searchPaths []string
    
    if s.config.Web.AssetsDir != "" {
        searchPaths = append(searchPaths, filepath.Join(s.config.Web.AssetsDir, filename))
    }
    
    fallbackPaths := []string{
        filepath.Join("web", filename),
        filepath.Join("./web", filename),
        filepath.Join("/usr/lib/raven/web", filename),
        filepath.Join("/opt/raven/web", filename),
    }
    searchPaths = append(searchPaths, fallbackPaths...)
    
    var listItems strings.Builder
    for _, path := range searchPaths {
        if _, err := os.Stat(path); err == nil {
            listItems.WriteString(fmt.Sprintf("<li><code>%s</code> ✅ (exists but not accessible)</li>", path))
        } else {
            listItems.WriteString(fmt.Sprintf("<li><code>%s</code> ❌ (not found)</li>", path))
        }
    }
    
    return listItems.String()
}

// Legacy methods for backward compatibility - these now use the new configurable system

func (s *Server) serveSPA(c *gin.Context) {
    rootFile := s.config.Web.Root
    if rootFile == "" {
        rootFile = "index.html"
    }
    s.serveConfiguredFile(c, rootFile)
}

func (s *Server) serveCSS(c *gin.Context) {
    s.serveConfiguredFile(c, "styles.css")
}

func (s *Server) serveFavicon(c *gin.Context) {
    s.serveConfiguredFile(c, "favicon.svg")
}

func (s *Server) serveFaviconICO(c *gin.Context) {
    s.serveConfiguredFile(c, "favicon.ico")
}

// Rest of the methods remain the same...

func (s *Server) getStats(c *gin.Context) {
    statuses, err := s.store.GetStatus(c.Request.Context(), database.StatusFilters{
        Limit: 1000,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status"})
        return
    }

    stats := map[string]int{
        "ok":       0,
        "warning":  0,
        "critical": 0,
        "unknown":  0,
    }

    for _, status := range statuses {
        switch status.ExitCode {
        case 0:
            stats["ok"]++
        case 1:
            stats["warning"]++
        case 2:
            stats["critical"]++
        default:
            stats["unknown"]++
        }
    }

    c.JSON(http.StatusOK, gin.H{"data": stats})
}

func (s *Server) getChecks(c *gin.Context) {
    checks, err := s.store.GetChecks(c.Request.Context())
    if err != nil {
        logrus.WithError(err).Error("Failed to get checks")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get checks"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "data":  checks,
        "count": len(checks),
    })
}

func (s *Server) getCheck(c *gin.Context) {
    id := c.Param("id")
    
    check, err := s.store.GetCheck(c.Request.Context(), id)
    if err != nil {
        if err.Error() == "check not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "Check not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get check"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"data": check})
}

// getWebConfig returns web configuration for the frontend
func (s *Server) getWebConfig(c *gin.Context) {
    config := gin.H{
        "header_link": s.config.Web.HeaderLink,
        "serve_static": s.config.Web.ServeStatic,
        "root": s.config.Web.Root,
    }
    
    c.JSON(http.StatusOK, gin.H{"data": config})
}

func (s *Server) getStatusHistory(c *gin.Context) {
    hostID := c.Param("host")
    checkID := c.Param("check")
    
    since := time.Now().Add(-24 * time.Hour)
    if sinceStr := c.Query("since"); sinceStr != "" {
        if parsedSince, err := time.Parse(time.RFC3339, sinceStr); err == nil {
            since = parsedSince
        }
    }

    history, err := s.store.GetStatusHistory(c.Request.Context(), hostID, checkID, since)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status history"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "data":  history,
        "count": len(history),
    })
}

func (s *Server) updateMetricsRoutine(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := s.metrics.UpdateSystemMetrics(ctx); err != nil {
                logrus.WithError(err).Error("Failed to update system metrics")
            }
        }
    }
}

func (s *Server) healthCheck(c *gin.Context) {
    health := gin.H{
        "status":    "healthy",
        "timestamp": time.Now(),
        "version":   Version,
        "services":  gin.H{},
    }
    
    services := health["services"].(gin.H)
    
    // Check database connectivity
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if _, err := s.store.GetHosts(ctx, database.HostFilters{}); err != nil {
        services["database"] = gin.H{
            "status": "unhealthy",
            "error":  err.Error(),
        }
        health["status"] = "degraded"
    } else {
        services["database"] = gin.H{"status": "healthy"}
    }
    
    // Check web assets
    missingFiles := []string{}
    foundFiles := []string{}
    
    filesToCheck := s.config.Web.Files
    if len(filesToCheck) == 0 {
        // Check default files if none configured
        filesToCheck = []string{"index.html", "styles.css", "favicon.ico"}
    }
    
    for _, filename := range filesToCheck {
        if s.findAssetFile(filename) != "" {
            foundFiles = append(foundFiles, filename)
        } else {
            missingFiles = append(missingFiles, filename)
        }
    }
    
    if len(missingFiles) == 0 {
        services["web_interface"] = gin.H{
            "status": "healthy",
            "found_files": foundFiles,
        }
    } else {
        services["web_interface"] = gin.H{
            "status": "degraded",
            "found_files": foundFiles,
            "missing_files": missingFiles,
        }
        if len(foundFiles) == 0 {
            health["status"] = "degraded"
        }
    }
    
    s.wsMu.Lock()
    activeClients := len(s.wsClients)
    s.wsMu.Unlock()

    services["websocket"] = gin.H{
        "status":         "healthy",
        "active_clients": activeClients,
    }
    
    services["monitoring"] = gin.H{"status": "healthy"}
    
    httpStatus := http.StatusOK
    if health["status"] == "degraded" {
        httpStatus = http.StatusServiceUnavailable
    }
    
    c.JSON(httpStatus, health)
}

func (s *Server) webDiagnostics(c *gin.Context) {
    diagnostics := gin.H{
        "timestamp": time.Now(),
        "configuration": gin.H{
            "assets_dir":    s.config.Web.AssetsDir,
            "static_dir":    s.config.Web.StaticDir,
            "serve_static":  s.config.Web.ServeStatic,
            "root":          s.config.Web.Root,
            "files":         s.config.Web.Files,
        },
        "web_assets": gin.H{},
    }

    // Check all configured files
    filesToCheck := s.config.Web.Files
    if len(filesToCheck) == 0 {
        filesToCheck = []string{"index.html", "styles.css", "favicon.ico", "favicon.svg"}
    }
    
    assetResults := make(map[string]interface{})
    
    for _, filename := range filesToCheck {
        var searchPaths []string
        
        if s.config.Web.AssetsDir != "" {
            configuredPath := filepath.Join(s.config.Web.AssetsDir, filename)
            searchPaths = append(searchPaths, configuredPath)
        }
        
        fallbackPaths := []string{
            filepath.Join("web", filename),
            filepath.Join("./web", filename),
            filepath.Join("/usr/lib/raven/web", filename),
            filepath.Join("/opt/raven/web", filename),
        }
        searchPaths = append(searchPaths, fallbackPaths...)
        
        pathResults := make([]gin.H, 0, len(searchPaths))
        
        for i, path := range searchPaths {
            result := gin.H{
                "path":     path,
                "priority": i + 1,
            }
            
            if i == 0 && s.config.Web.AssetsDir != "" {
                result["source"] = "configured"
            } else {
                result["source"] = "default"
            }
            
            if stat, err := os.Stat(path); err == nil {
                result["exists"] = true
                result["size"] = stat.Size()
                result["modified"] = stat.ModTime()
                result["readable"] = true
                
                // For HTML files, check if they look valid
                if strings.HasSuffix(filename, ".html") {
                    if file, err := os.Open(path); err == nil {
                        buffer := make([]byte, 200)
                        if n, err := file.Read(buffer); err == nil {
                            content := string(buffer[:n])
                            result["looks_like_html"] = strings.Contains(strings.ToLower(content), "<!doctype html") || 
                                                       strings.Contains(strings.ToLower(content), "<html")
                            result["preview"] = content
                        }
                        file.Close()
                    }
                }
            } else {
                result["exists"] = false
                result["error"] = err.Error()
            }
            
            pathResults = append(pathResults, result)
        }
        
        assetResults[filename] = gin.H{
            "paths": pathResults,
            "configured": contains(s.config.Web.Files, filename),
        }
    }
    
    diagnostics["web_assets"] = assetResults
    
    if cwd, err := os.Getwd(); err == nil {
        diagnostics["working_directory"] = cwd
    }
    
    c.JSON(http.StatusOK, diagnostics)
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}

func corsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*")
        c.Header("Access-Control-Allow-Credentials", "true")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
        c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    }
}


