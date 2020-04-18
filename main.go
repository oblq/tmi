package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// should be interpolated with -ldflags at build time.
var Path = "/home/marco/go/src/github.com/oblq/tmi/artifacts/"

func main() {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)
	go func() {
		stop := <-stopCh
		fmt.Printf("\n%v\n", stop)
		done <- true
	}()

	cm, err := New(Path)
	if err != nil {
		panic(err)
	}
	if err = cm.LoadConfigAndStart(); err != nil {
		panic(err)
	}

	<-done
	fmt.Println("exiting")

	cm.StopMonitoring()
	for _, c := range cm.closers {
		c.Close()
	}
}
