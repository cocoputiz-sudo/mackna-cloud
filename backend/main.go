package main

import (
    "bufio"
    "context"
    "crypto/subtle"
    "embed"
    "encoding/json"
    "errors"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "os/exec"
    "runtime"
    "strconv"
    "strings"
    "syscall"
    "time"
)

//go:embed dashboard.html
var dashboardHTML embed.FS

const version = "0.1.0"
const listenAddress = "127.0.0.1:8090" // Never expose HTTP Basic authentication directly to the LAN.
var page = template.Must(template.New("dashboard.html").Funcs(template.FuncMap{"human": humanBytes}).ParseFS(dashboardHTML, "dashboard.html"))

func humanBytes(n uint64) string {
    const unit = 1024
    if n < unit { return fmt.Sprintf("%d B", n) }
    v := float64(n)
    suffix := []string{"KiB", "MiB", "GiB", "TiB"}
    for _, unitName := range suffix {
        v /= unit
        if v < unit { return fmt.Sprintf("%.1f %s", v, unitName) }
    }
    return fmt.Sprintf("%.1f PiB", v / unit)
}

type systemInfo struct {
    Hostname       string  `json:"hostname"`
    Architecture   string  `json:"architecture"`
    UptimeSeconds  float64 `json:"uptime_seconds"`
    LoadAverage1m  float64 `json:"load_average_1m"`
    MemoryTotal    uint64  `json:"memory_total_bytes"`
    MemoryAvailable uint64 `json:"memory_available_bytes"`
    DiskTotal      uint64  `json:"disk_total_bytes"`
    DiskAvailable  uint64  `json:"disk_available_bytes"`
}

type containerInfo struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Image  string `json:"image"`
    State  string `json:"state"`
    Status string `json:"status"`
}

type agent struct {
    token      string
    system     func() (systemInfo, error)
    containers func(context.Context) ([]containerInfo, error)
}

func readSystem() (systemInfo, error) {
    info := systemInfo{}
    hostname, err := os.Hostname()
    if err != nil { return info, err }
    info.Hostname = hostname
    info.Architecture = "linux/" + runtimeArch()
    uptime, err := os.ReadFile("/proc/uptime")
    if err != nil { return info, err }
    uptimeFields := strings.Fields(string(uptime))
    if len(uptimeFields) < 1 { return info, errors.New("invalid /proc/uptime") }
    info.UptimeSeconds, err = strconv.ParseFloat(uptimeFields[0], 64)
    if err != nil { return info, err }
    load, err := os.ReadFile("/proc/loadavg")
    if err != nil { return info, err }
    loadFields := strings.Fields(string(load))
    if len(loadFields) < 1 { return info, errors.New("invalid /proc/loadavg") }
    info.LoadAverage1m, err = strconv.ParseFloat(loadFields[0], 64)
    if err != nil { return info, err }
    mem, err := os.Open("/proc/meminfo")
    if err != nil { return info, err }
    defer mem.Close()
    scanner := bufio.NewScanner(mem)
    var totalFound, availableFound bool
    for scanner.Scan() {
        fields := strings.Fields(scanner.Text())
        if len(fields) < 2 { continue }
        if fields[0] == "MemTotal:" || fields[0] == "MemAvailable:" {
            value, parseErr := strconv.ParseUint(fields[1], 10, 64)
            if parseErr != nil { return info, parseErr }
            if fields[0] == "MemTotal:" { info.MemoryTotal = value * 1024; totalFound = true }
            if fields[0] == "MemAvailable:" { info.MemoryAvailable = value * 1024; availableFound = true }
        }
    }
    if err := scanner.Err(); err != nil { return info, err }
    if !totalFound || !availableFound { return info, errors.New("missing memory metrics") }
    var stat syscall.Statfs_t
    if err := syscall.Statfs("/", &stat); err != nil { return info, err }
    info.DiskTotal = stat.Blocks * uint64(stat.Bsize)
    info.DiskAvailable = stat.Bavail * uint64(stat.Bsize)
    return info, nil
}

func runtimeArch() string {
    return runtime.GOARCH
}

func readContainers(ctx context.Context) ([]containerInfo, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    // Fixed CLI arguments, no shell and no user-supplied Docker subcommands.
    cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{json .}}")
    output, err := cmd.Output()
    if err != nil { return nil, errors.New("docker inventory unavailable") }
    if len(output) > 2<<20 { return nil, errors.New("docker inventory too large") }
    list := make([]containerInfo, 0)
    scanner := bufio.NewScanner(strings.NewReader(string(output)))
    scanner.Buffer(make([]byte, 4096), 128*1024)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" { continue }
        var record map[string]string
        if err := json.Unmarshal([]byte(line), &record); err != nil { return nil, err }
        list = append(list, containerInfo{
            ID: record["ID"], Name: record["Names"], Image: record["Image"],
            State: record["State"], Status: record["Status"],
        })
        if len(list) > 500 { return nil, errors.New("too many containers") }
    }
    if err := scanner.Err(); err != nil { return nil, err }
    return list, nil
}

func newAgent(token string) *agent {
    return &agent{token: token, system: readSystem, containers: readContainers}
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(value)
}

func (a *agent) authorized(r *http.Request) bool {
    username, password, ok := r.BasicAuth()
    if !ok { return false }
    userOK := subtle.ConstantTimeCompare([]byte(username), []byte("admin")) == 1
    passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.token)) == 1
    return userOK && passOK
}

func (a *agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.Header().Set("X-Frame-Options", "DENY")
    w.Header().Set("Referrer-Policy", "no-referrer")
    w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
    if r.Method != http.MethodGet {
        w.Header().Set("Allow", "GET")
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    if r.URL.Path == "/api/v1/health" {
        jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "version": version})
        return
    }
    if !a.authorized(r) {
        w.Header().Set("WWW-Authenticate", `Basic realm="MackNa Cloud Node 01", charset="UTF-8"`)
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    switch r.URL.Path {
    case "/api/v1/system":
        info, err := a.system()
        if err != nil { jsonResponse(w, 503, map[string]string{"error": "system_metrics_unavailable"}); return }
        jsonResponse(w, 200, info)
    case "/api/v1/containers":
        containers, err := a.containers(r.Context())
        if err != nil { jsonResponse(w, 503, map[string]string{"error": "docker_inventory_unavailable"}); return }
        jsonResponse(w, 200, containers)
    case "/":
        info, sysErr := a.system()
        containers, dockerErr := a.containers(r.Context())
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        data := struct {
            Node string
            Info systemInfo
            SystemOK bool
            DockerOK bool
            Containers []containerInfo
            Version string
        }{ Node: "Node 01", Info: info, SystemOK: sysErr == nil,
           DockerOK: dockerErr == nil, Containers: containers, Version: version }
        if err := page.Execute(w, data); err != nil { log.Printf("dashboard render error: %v", err) }
    default:
        http.NotFound(w, r)
    }
}

func main() {
    token := os.Getenv("MACKNA_ADMIN_TOKEN")
    if len(token) < 32 {
        log.Fatal("MACKNA_ADMIN_TOKEN must be a randomly generated secret of at least 32 characters")
    }
    srv := &http.Server{
        Addr: listenAddress,
        Handler: newAgent(token),
        ReadHeaderTimeout: 5*time.Second,
        WriteTimeout: 10*time.Second,
        IdleTimeout: 30*time.Second,
        MaxHeaderBytes: 8192,
    }
    log.Printf("MackNa Cloud agent %s listening on %s (SSH tunnel only)", version, listenAddress)
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatal(fmt.Errorf("server stopped: %w", err))
    }
}
