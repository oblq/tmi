package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// can be interpolated with -ldflags at build time with an absolute path.
var Path = "./"

//func init() {
//	if Path == "" {
//		//_, filename, _, _ := runtime.Caller(0)
//		//Path = filepath.Join(path.Dir(filename), "artifacts/")
//	}
//}

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

	<-done
	fmt.Println("exiting")

	cm.StopMonitoring()
	for _, c := range cm.plugins {
		c.ShutDown()
	}
}
