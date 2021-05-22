package serializer

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobSerializer struct {
	conn io.ReadWriteCloser
	buf *bufio.Writer
	dec *gob.Decoder
	enc *gob.Encoder
}

var _ Serializer = (*GobSerializer)(nil)

func NewGobSerializer (conn io.ReadWriteCloser) Serializer {
	buf := bufio.NewWriter(conn)
	return &GobSerializer{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

func (c *GobSerializer) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

func (c *GobSerializer) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobSerializer) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	if err := c.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header:", err)
		return err
	}
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (c *GobSerializer) Close() error {
	return c.conn.Close()
}