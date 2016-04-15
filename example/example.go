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
	srv.Run()
}

func IsPidActive() (int, string) {
	p, err := os.FindProcess(13755)

	if err != nil {
		return httphealth.STATUS_CRITICAL, err.Error()
	}

	return httphealth.STATUS_OK, strconv.Itoa(p.Pid)
}

func FailingCheck() (int, string) {
	return httphealth.STATUS_CRITICAL, "something bad"
}
