package g33rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SeanChan0901/g33rpc/serializer"
	"io"
	"log"
	"net"
	"sync"
)

// Call represents an active RPC.
type Call struct {
	Seq				uint64
	ServiceMethod 	string			// format "<service>.<method>"
	Args  			interface{}		// arguments to the function
	Reply 			interface{}		// reply from the function
	Error 			error			// if error occurs, it will be set
	Done 			chan *Call		// Strobes when call is complete.
}

func (call *Call) done() {
	call.Done <- call
}

// Client represents an RPC Client.
// There may be multiple outstanding Calls associated
// with a single Client, and a Client may be used by
// multiple goroutines simultaneously.
type Client struct {
	ss 			serializer.Serializer
	opt 		*Option
	sending 	sync.Mutex  // protect following
	header 		serializer.Header
	mu 			sync.Mutex  // protect following
	seq			uint64
	pending 	map[uint64]*Call
	closing 	bool		// user has called Close
	shutdown 	bool		// server has told us to stop
}

var _ io.Closer = (*Client)(nil)

var ErrShutdonw = errors.New("connection is shut down")

// Close the connection
func (client * Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdonw
	}
	client.closing = true
	return client.ss.Close()
}

// IsAvailable return true if the client dose work
func (client *Client) isAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing  // not shutdown and not closing
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutdonw
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq ++
	return call.Seq, nil
}

func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

func (client *Client) terminateCall(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func (client *Client) receive() {
	var err error
	for err == nil {
		var h serializer.Header
		if err = client.ss.ReadHeader(&h); err != nil {
			break;
		}
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			// it usually means that Write partially failed
			// and call was already removed.
			err = client.ss.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = client.ss.ReadBody(nil)
			call.done()
		default:
			err = client.ss.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body" + err.Error())
			}
			call.done()
		}
	}
	// error occurs, so terminateCalls pending calls
	client.terminateCall(err)
}

func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	f := serializer.NewSerializerMap[opt.CodeType]
	if f == nil {
		err := fmt.Errorf("invalid CodeType %s", opt.CodeType)
		log.Println("rpc client : serializer error", err)
		_ = conn.Close()
		return nil, err
	}
	// send options with server
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client : options error:", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientSerializer(f(conn), opt), nil
}

func newClientSerializer(ss serializer.Serializer, opt *Option) *Client {
	client := &Client{
		seq: 		1,
		ss:			ss,
		opt: 		opt,
		pending: 	make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

func parseOptions(opts ...*Option) (*Option, error) {
	// if opts is nil or pass nil as parameter
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}

	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodeType == "" {
		opt.CodeType = DefaultOption.CodeType
	}
	return opt, nil
}

// Dial connects to an RPC server at the specified network address
func Dial(network, address string, opts ...*Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	// close the connection if client is nil
	defer func() {
		if client == nil {
			_ = conn.Close
		}
	}()
	return NewClient(conn, opt)
}

func (client *Client) send(call *Call) {
	// make sure that the client will send a complete request
	client.sending.Lock()
	defer client.sending.Unlock()

	// register this call
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare request header
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// encode and send the request
	if err := client.ss.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		// call may be nil, it usually means that Write partially failed
		// client has received the response and handle
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Go invokes the function asynchronously
// It returns the Call structure representing the invocation
func (client *Client) Go(serviceMethod string, args,  reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client channel is unbuffered")
	}

	call := &Call {
		ServiceMethod : serviceMethod,
		Args : 			args,
		Reply:			reply,
		Done:			done,
	}
	client.send(call)
	return call
}

// Call invokes the named function, waits for it to complete,
// and returns its error status.
func (client *Client) Call(serviceMethod string, args, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}






