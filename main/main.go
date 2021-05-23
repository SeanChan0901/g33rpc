package main

import (
	"encoding/json"
	"fmt"
	"github.com/SeanChan0901/g33rpc"
	"github.com/SeanChan0901/g33rpc/serializer"
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
	g33rpc.Accept(listener)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	conn, _ := net.Dial("tcp", <-addr)
	defer func(){_ = conn.Close()}()

	time.Sleep(time.Second)
	_ = json.NewEncoder(conn).Encode(g33rpc.DefaultOption)
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