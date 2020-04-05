package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// should be interpolated with -ldflags at build time.
var Path = "/home/marco/go/src/github.com/oblq/ipmifc/artifacts/"

func main() {
	ipmifc, err := New(Path)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		stop := <-stopCh
		fmt.Printf("\n%v\n", stop)
		done <- true
	}()

	<-done
	fmt.Println("exiting")

	ipmifc.StopMonitoring()
	ipmifc.SetFanMode(fanModeOptimal)
}
