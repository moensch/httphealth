package main

import (
	"fmt"
	"github.com/moensch/httphealth"
	"os"
)

func main() {
	srv, err := httphealth.NewHttpHealth()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("initialized")
	srv.Run()
}
