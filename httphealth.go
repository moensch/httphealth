package httphealth

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"io"
	"net/http"
	"strings"
)

const (
	STATUS_OK       = 0
	STATUS_WARN     = 1
	STATUS_CRITICAL = 2
	STATUS_UNKNOWN  = 3
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
}

type listenConfig struct {
	Port    int
	Address string
}

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}

func NewHttpHealth() (*HttpHealth, error) {
	httphealth := &HttpHealth{
		config: tomlConfig{
			Listen: listenConfig{
				Port:    8000,
				Address: "0.0.0.0",
			},
		},
	}
	if _, err := toml.DecodeFile("config.toml", &httphealth.config); err != nil {
		return httphealth, err
	}

	var err error
	return httphealth, err
}

func (h *HttpHealth) Run() {
	http.HandleFunc("/", h.RunAllChecks)
	http.HandleFunc("/checks", h.ListChecks)
	http.HandleFunc("/checks/", h.RunCheck)
	http.ListenAndServe(":8000", nil)
}

func (h *HttpHealth) ListChecks(w http.ResponseWriter, r *http.Request) {
	for checkName, _ := range h.checks {
		io.WriteString(w, fmt.Sprintf("%s\n", checkName))
	}
}

type CheckResponse struct {
	text   string
	status int
}

func (resp *CheckResponse) IsCritical() bool {
	return resp.status == STATUS_CRITICAL
}

func (resp *CheckResponse) IsOk() bool {
	return resp.status == STATUS_OK
}

func (resp *CheckResponse) IsWarn() bool {
	return resp.status == STATUS_WARN
}

func (resp *CheckResponse) IsUnknown() bool {
	return resp.status == STATUS_UNKNOWN
}

func (resp *CheckResponse) StatusText() string {
	switch resp.status {
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
	}{
		Text:       resp.text,
		StatusCode: resp.status,
		Status:     resp.StatusText(),
	})
}

func (h *HttpHealth) RunAllChecks(w http.ResponseWriter, r *http.Request) {
	var results map[string]CheckResponse

	results = make(map[string]CheckResponse)

	for checkName, check := range h.checks {
		fmt.Println("Running check " + checkName)
		status, response := check.f.Run()
		fmt.Printf("Status: %d\n", status)
		fmt.Printf("Response: %s\n", response)
		results[checkName] = CheckResponse{
			text:   response,
			status: status,
		}
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

func (h *HttpHealth) RunCheck(w http.ResponseWriter, r *http.Request) {
	urlparts := strings.Split(r.URL.String(), "/")
	checkname, _ := urlparts[len(urlparts)-1], urlparts[:len(urlparts)-1]
	if val, ok := h.checks[checkname]; ok {
		fmt.Printf("Running check: %s\n", val.name)
		status, response := val.f.Run()
		resp := CheckResponse{
			text:   response,
			status: status,
		}
		b, err := json.Marshal(resp)
		if err != nil {
			fmt.Printf("ERROR with JSON: %s", err)
			http.Error(w, "Cannot produce JSON response", 503)
			return
		}
		if status != STATUS_OK {
			http.Error(w, string(b), 503)
			return
		}
		io.WriteString(w, string(b))
	} else {
		http.NotFound(w, r)
		return
	}
}

type CheckFunc func() (int, string)

func (f CheckFunc) Run() (int, string) {
	return f()
}

type Check interface {
	Run() (int, string)
}

func (h *HttpHealth) RegisterCheck(name string, check func() (int, string)) error {
	var err error
	fmt.Println("Registering check " + name)
	h.Register(name, CheckFunc(check))
	return err
}

type CheckEntry struct {
	name string
	f    Check
}

func (h *HttpHealth) Register(name string, check Check) {
	fmt.Println("Registering: " + name)
	if h.checks == nil {
		h.checks = make(map[string]CheckEntry)
	}

	h.checks[name] = CheckEntry{
		name: name,
		f:    check,
	}
}
