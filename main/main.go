package main

import (
	"context"
	"github.com/SeanChan0901/g33rpc"
	"log"
	"net"
	"sync"
	"time"
)

type Foo int

type Args struct {Num1, Num2 int}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func startServer(addr chan string) {
	var foo Foo
	if err := g33rpc.Register(&foo); err != nil {
		log.Fatal("register error", err)
	}

	// pick a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc on", listener.Addr())
	addr <- listener.Addr().String()
	g33rpc.Accept(listener)
}

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	go startServer(addr)
	client, _ := g33rpc.Dial("tcp", <-addr)
	defer func() {_ = client.Close()}()

	time.Sleep(time.Second)
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i ++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1 : i, Num2 : i * i}
			var reply int
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error")
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}