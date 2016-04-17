package httphealth

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	STATUS_OK       = 0
	STATUS_WARN     = 1
	STATUS_CRITICAL = 2
	STATUS_UNKNOWN  = 3
)

var (
	CheckCache    Cache
	ListenPort    int
	ListenAddress string
	ConfigFile    string
)

type HttpHealth struct {
	config tomlConfig
	checks map[string]CheckEntry
}

type HealthCheck struct {
	name string
}

type tomlConfig struct {
	Listen listenConfig
	Checks map[string]checkConfig
}

type listenConfig struct {
	Port    int
	Address string
}

type checkConfig struct {
	Command string
	Cache   string
}

func (c checkConfig) Run() CheckResponse {
	resp := CheckResponse{
		Status: STATUS_OK,
	}

	fmt.Printf("Running command: %s\n", c.Command)
	cmd := exec.Command("sh", "-c", c.Command)

	cmdOutput, err := cmd.CombinedOutput()
	resp.Text = string(cmdOutput)
	if err != nil {
		resp.Status = STATUS_CRITICAL
	}

	return resp
}

func init() {
	flag.StringVar(&ListenAddress, "l", "", "Interface to listen on")
	flag.IntVar(&ListenPort, "p", 0, "Port to listen on")
	flag.StringVar(&ConfigFile, "c", "", "Configuration File")
}

func NewHttpHealth() (*HttpHealth, error) {
	flag.Parse()
	httphealth := &HttpHealth{
		config: tomlConfig{
			Listen: listenConfig{
				Port:    8000,
				Address: "0.0.0.0",
			},
		},
	}

	configLocations := make([]string, 0)

	if ConfigFile != "" {
		// Config file specified on command line
		configLocations = append(configLocations, ConfigFile)
	} else {
		// Default config search paths
		configLocations = append(configLocations, "/etc/httphealth.toml")
		configLocations = append(configLocations, "/httphealth.toml")
		configLocations = append(configLocations, "config.toml")
	}

	// Parse config if exists in any of our searhc locations
	for _, configpath := range configLocations {
		fmt.Printf("Checking for config in %s\n", configpath)
		if _, err := os.Stat(configpath); err == nil {
			err = httphealth.LoadConfig(configpath)
			if err != nil {
				return httphealth, err
			}
			break
		}
	}

	// Listen address from command line
	if ListenAddress != "" {
		// Takes precedence over config file
		httphealth.config.Listen.Address = ListenAddress
	}

	// Listen port from command line
	if ListenPort != 0 {
		// Takes precedence over config file
		httphealth.config.Listen.Port = ListenPort
	}

	// Initialize ad-hoc checks defined in config file
	if httphealth.config.Checks == nil {
		httphealth.config.Checks = make(map[string]checkConfig)
	}

	// Register command line checks from config
	for name, entry := range httphealth.config.Checks {
		if entry.Cache != "" {
			duration, err := time.ParseDuration(entry.Cache)
			if err != nil {
				return httphealth, err
			}
			fmt.Printf("  Check %s has cache %s (%d)\n", name, entry.Cache, int64(duration.Seconds()))
			httphealth.RegisterCachingCheck(name, entry.Run, int64(duration.Seconds()))
		} else {
			httphealth.RegisterCheck(name, entry.Run)
		}
	}

	// Initialize cache
	CheckCache = Cache{}

	var err error
	return httphealth, err
}

func (h *HttpHealth) LoadConfig(path string) error {
	fmt.Printf("Parsing config file: %s\n", path)
	_, err := toml.DecodeFile(path, &h.config)
	return err
}

func (h *HttpHealth) Run() {
	http.HandleFunc("/", h.RunAllChecks)
	http.HandleFunc("/checks", h.HandleChecks)
	http.HandleFunc("/checks/", h.HandleChecks)
	http.ListenAndServe(fmt.Sprintf("%s:%d", h.config.Listen.Address, h.config.Listen.Port), nil)
}

type CheckResponse struct {
	Text      string
	Status    int
	FromCache bool
	CacheTtl  int64
}

func (resp *CheckResponse) IsCritical() bool {
	return resp.Status == STATUS_CRITICAL
}

func (resp *CheckResponse) IsOk() bool {
	return resp.Status == STATUS_OK
}

func (resp *CheckResponse) IsWarn() bool {
	return resp.Status == STATUS_WARN
}

func (resp *CheckResponse) IsUnknown() bool {
	return resp.Status == STATUS_UNKNOWN
}

func (resp *CheckResponse) StatusText() string {
	switch resp.Status {
	case STATUS_OK:
		return "ok"
	case STATUS_WARN:
		return "warning"
	case STATUS_CRITICAL:
		return "critical"
	case STATUS_UNKNOWN:
		return "unknown"
	default:
		return "critical"
	}
}

func (resp CheckResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Text       string `json:"text"`
		StatusCode int    `json:"status_code"`
		Status     string `json:"status"`
		FromCache  bool   `json:"cache_used"`
		CacheTtl   int64  `json:"cache_ttl"`
	}{
		Text:       resp.Text,
		StatusCode: resp.Status,
		Status:     resp.StatusText(),
		FromCache:  resp.FromCache,
		CacheTtl:   resp.CacheTtl,
	})
}

func (h *HttpHealth) RunAllChecks(w http.ResponseWriter, r *http.Request) {
	var results map[string]CheckResponse

	results = make(map[string]CheckResponse)

	for checkName, check := range h.checks {
		//fmt.Println("Running check " + checkName)
		//results[checkName] = check.f.Run()
		results[checkName] = check.Run()
		//fmt.Printf("Status: %d\n", results[checkName].Status)
		//fmt.Printf("Response: %s\n", results[checkName].Text)
	}

	var hasError bool
	for _, result := range results {
		if !result.IsOk() {
			hasError = true
		}
	}
	b, err := json.Marshal(results)
	if err != nil {
		fmt.Printf("ERROR with JSON: %s", err)
		http.Error(w, "Cannot produce JSON response", 503)
		return
	}
	if hasError {
		http.Error(w, string(b), 503)
	} else {
		io.WriteString(w, string(b))
	}
}

func (h *HttpHealth) HandleChecks(w http.ResponseWriter, r *http.Request) {
	urlparts := strings.Split(r.URL.String(), "/")

	if len(urlparts) == 2 || (len(urlparts) == 3 && urlparts[2] == "") {
		// Return a listing, no check specified
		for checkName, _ := range h.checks {
			io.WriteString(w, fmt.Sprintf("%s\n", checkName))
		}
	} else {
		// Run specified check
		h.RunCheck(w, r, urlparts[2])
	}
}

func (h *HttpHealth) RunCheck(w http.ResponseWriter, r *http.Request, checkname string) {
	if val, ok := h.checks[checkname]; ok {
		fmt.Printf("Running check: %s\n", val.name)
		resp := val.Run()
		b, err := json.Marshal(resp)
		if err != nil {
			fmt.Printf("ERROR with JSON: %s", err)
			http.Error(w, "Cannot produce JSON response", 503)
			return
		}
		if !resp.IsOk() {
			http.Error(w, string(b), 503)
			return
		}
		io.WriteString(w, string(b))
	} else {
		http.NotFound(w, r)
		return
	}
}

type CheckFunc func() CheckResponse

func (f CheckFunc) Run() CheckResponse {
	return f()
}

type Check interface {
	Run() CheckResponse
}

func (h *HttpHealth) RegisterCheck(name string, check func() CheckResponse) error {
	var err error
	fmt.Println("Registering check " + name)
	h.Register(name, CheckFunc(check), 0)
	return err
}

func (h *HttpHealth) RegisterCachingCheck(name string, check func() CheckResponse, ttl int64) error {
	var err error
	fmt.Println("Registering caching check " + name)
	h.Register(name, CheckFunc(check), ttl)
	return err
}

type CheckEntry struct {
	name     string
	f        Check
	cacheTtl int64
}

func (e *CheckEntry) Run() CheckResponse {
	if e.cacheTtl == 0 {
		return e.f.Run()
	}

	// Query the cache
	resp, validFor, err := CheckCache.Get(e.name)
	if err == nil {
		// Cache hit
		resp.FromCache = true
		resp.CacheTtl = validFor
		return resp
	}

	// Run the check
	resp = e.f.Run()

	// Cache the result
	CheckCache.Set(e.name, resp, e.cacheTtl)

	return resp
}

func (h *HttpHealth) Register(name string, check Check, cacheTtl int64) {
	fmt.Println("Registering: " + name)
	if h.checks == nil {
		h.checks = make(map[string]CheckEntry)
	}

	h.checks[name] = CheckEntry{
		name:     name,
		f:        check,
		cacheTtl: cacheTtl,
	}
}
