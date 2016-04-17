package main

import (
	"fmt"
	"github.com/moensch/httphealth"
	"os"
	"strconv"
)

type PidActive struct {
	name string
}

func main() {
	srv, err := httphealth.NewHttpHealth()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	srv.RegisterCheck("pidactive", IsPidActive)
	srv.RegisterCheck("failing", FailingCheck)
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
