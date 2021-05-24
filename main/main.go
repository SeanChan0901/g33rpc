package main

import (
	"fmt"
	"github.com/SeanChan0901/g33rpc"
	"log"
	"net"
	"sync"
	"time"
)

func startServer(addr chan string) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", listener.Addr())
	addr <- listener.Addr().String()
	g33rpc.Accept(listener)
}

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	go startServer(addr)
	client, _ := g33rpc.Dial("tcp", <-addr)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second * 5)
	// send requeset & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i ++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("g33rpc req %d : ", i)
			var reply string
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}