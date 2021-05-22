package geerpc

import (
	"encoding/json"
	"fmt"
	"github.com/SeanChan0901/geerpc/serializer"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

type Option struct {
	CodeType    serializer.Type
	MagicNumber int
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodeType:    serializer.GobType,
}

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

func (server *Server) Accept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go server.ServeConn(conn)
	}
}

func Accept(listener net.Listener) {
	DefaultServer.Accept(listener)
}

func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}

	if opt.MagicNumber != MagicNumber {
		log.Println("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}

	f := serializer.NewSerializerMap[opt.CodeType]
	if f == nil {
		log.Println("rpc server: invalid codec type %s", opt.CodeType)
		return
	}

	server.serveHandler(f(conn))
}

var invalidRequest = struct{}{}

func (server *Server) serveHandler(ss serializer.Serializer) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		req, err := server.readRequest(ss)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(ss, req.h, invalidRequest, sending)
		}
		wg.Add(1)
		go server.handleRequest(ss, req, sending, wg)
	}
	wg.Wait()
	_ = ss.Close()
}

type request struct {
	h *serializer.Header
	argv, replyv reflect.Value
}

func (server *Server) readRequestHeader(ss serializer.Serializer) (*serializer.Header, error) {
	var h serializer.Header
	if err := ss.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRequest(ss serializer.Serializer) (*request, error) {
	h, err := server.readRequestHeader(ss)
	if err != nil {
		return nil, err
	}

	req := request{h : h}
	req.argv = reflect.New(reflect.TypeOf(""))
	if err := ss.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return &req, nil
}

func (server *Server) sendResponse(ss serializer.Serializer, h *serializer.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := ss.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(ss serializer.Serializer, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc response : %d", req.h.Seq))
	server.sendResponse(ss, req.h, req.replyv.Interface(), sending)
}