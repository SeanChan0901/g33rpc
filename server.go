package g33rpc

import (
	"encoding/json"
	"errors"
	"github.com/SeanChan0901/g33rpc/serializer"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
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

// Server represents an RPC Server.
type Server struct{
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

// Register publishes in the server the set of methods of the receiver
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc : service already defined :" + s.name)
	}
	return nil
}

// Register publishes the receiver's methods in the DefaultServer.
func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server : service/method request ill-format: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot + 1:]
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server : can't find service" + serviceName)
		return
	}
	svc = svci.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server : can't find method" + methodName)
	}
	return
}

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
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}

	f := serializer.NewSerializerMap[opt.CodeType]
	if f == nil {
		log.Printf("rpc server: invalid serializer type %s", opt.CodeType)
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
	h 				*serializer.Header
	argv, replyv 	reflect.Value
	mtype 			*methodType
	svc 			*service
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

	req := &request{h : h}
	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	// make sure that argvi is a pointer, ReadBody need a pointer as parameter
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}

	if err = ss.ReadBody(argvi); err != nil {
		log.Println("rpc server : read body err : ", err)
		return req, err
	}

	return req, nil
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
	err := req.svc.call(req.mtype, req.argv, req.replyv)
	if err != nil {
		req.h.Error = err.Error()
		server.sendResponse(ss, req.h, invalidRequest, sending)
		return
	}
	server.sendResponse(ss, req.h, req.replyv.Interface(), sending)
}