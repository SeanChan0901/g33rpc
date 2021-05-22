package main

import (
	"encoding/json"
	"fmt"
	"github.com/SeanChan0901/geerpc"
	"github.com/SeanChan0901/geerpc/serializer"
	"log"
	"net"
	"time"
)

func startServer(addr chan string) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", listener.Addr())
	addr <- listener.Addr().String()
	geerpc.Accept(listener)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	conn, _ := net.Dial("tcp", <-addr)
	defer func(){_ = conn.Close()}()

	time.Sleep(time.Second)
	_ = json.NewEncoder(conn).Encode(geerpc.DefaultOption)
	ss := serializer.NewGobSerializer(conn)
	for i := 0; i < 5; i ++ {
		time.Sleep(time.Second)
		h := &serializer.Header{
			ServiceMethod: "Foo.Sum",
			Seq: uint64(i),
		}
		_ = ss.Write(h, fmt.Sprintf("geerpc request %d", h.Seq))
		_ = ss.ReadHeader(h)
		var reply string
		_ = ss.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}