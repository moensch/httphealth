# httphealth
Simplistic pluggable HTTP server for reporting application and container health status

## Building
```bash
go get github.com/moensch/httphealth
```

Standalone server
```bash
go get github.com/moensch/httphealth/cmd/httphealthd
```

## Usage

### Health checks written in go (preferred)
```Go
package main

import (
        "github.com/moensch/httphealth"
        "fmt"
        "os"
)


func main() {
        srv, err := httphealth.NewHttpHealth()
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }

        srv.RegisterCheck("pidactive", IsPidActive)
        srv.RegisterCheck("thisfails", FailingCheck)
        srv.RegisterCachingCheck("cachethis", SomeCheck, 300)

        srv.Run()
}

func IsPidActive() httphealth.CheckResponse {
        resp := httphealth.CheckResponse{}

        p, err := os.FindProcess(13755)

        if err != nil {
                resp.Status = httphealth.STATUS_CRITICAL
                resp.Text = err.Error()

                return resp
        }

        resp.Status = httphealth.STATUS_OK
        resp.Text = strconv.Itoa(p.Pid)

        return resp
}

func FailingCheck() httphealth.CheckResponse {
        resp := httphealth.CheckResponse{
                Status: httphealth.STATUS_CRITICAL,
                Text:   "Some error message",
        }

        return resp
}

func SomeCheck() httphealth.CheckResponse {
        resp := httphealth.CheckResponse{
                Status: httphealth.STATUS_WARN,
                Text:   "this failed and is cached for 300 seconds",
        }

        return resp
}
```

### Using Standalone server

If you don't want to write your checks in Go, you can use shell commands as checks too.
Any non-zero exit code is considered a failure. All commands are run through /bin/sh -c.

You can use configuration file defined checks and check functions together.

**cache**: Duration string as defined here: https://golang.org/pkg/time/#ParseDuration

By default, no results are cached.

```Toml
[listen]
port = 9000
address = "0.0.0.0"

[checks]

[checks.apache]
command = "ps aux | grep [h]ttp"
cache = "5m"

[checks.failcmd]
command = "/bin/false"
```

## Running
```bash
#> httphealthd -c /etc/yourconf.toml
```

### Command line arguments
**-p** Listen port (overrides config /listen/port)
**-l** Listen address (overrides config /listen/address)
**-c** Configuration file path (default search paths: */etc/httphealth.toml*, */httphealth.toml*)

## Querying

**Response Codes:**
**250** All checks passed
**503** One or more checks are not STATUS_OK
**404** Only used when running single check, and check not found

### Run all checks
```bash
curl -s localhost:9000
```

**Response**
```json
{
    "cachethis": {
        "cache_ttl": 142,
        "cache_used": true,
        "status": "warning",
        "status_code": 1,
        "text": "this failed and is cached for 300 seconds"
    },
    "failing": {
        "cache_ttl": 0,
        "cache_used": false,
        "status": "critical",
        "status_code": 2,
        "text": "Some error message"
    },
    "pidactive": {
        "cache_ttl": 0,
        "cache_used": false,
        "status": "ok",
        "status_code": 0,
        "text": "13755"
    }
}
```
### Run single check
```bash
curl -s localhost:9000/checks/pidactive
```
**Response**
```json
{
    "cache_ttl": 0,
    "cache_used": false,
    "status": "ok",
    "status_code": 0,
    "text": "13755"
}
```
### List checks
```bash
curl -s localhost:9000/checks
```

**Response**
```
pidactive
failing
cachethis
```


