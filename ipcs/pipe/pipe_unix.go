// (c) 2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pipe

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

type Socket struct {
	connLock sync.RWMutex
	conns    []net.Conn

	listeningCh chan (struct{})
	quitCh      chan (struct{})
	doneCh      chan (struct{})
}

func NewSocket() *Socket {
	return &Socket{
		listeningCh: make(chan (struct{})),
		quitCh:      make(chan (struct{})),
		doneCh:      make(chan (struct{})),
	}
}

func (s *Socket) Listen(addr string) error {
	// unixAddr, err := net.ResolveUnixAddr("unix", addr)
	// if err != nil {
	// 	return err
	// }

	l, err := net.Listen("unix", addr)

	// l, err := net.ListenUnix("unix", unixAddr)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-s.quitCh:
				close(s.doneCh)
				return
			default:
				s.accept(l)
			}
		}
	}()

	return nil
}

func (s *Socket) Send(msg []byte) error {
	// Serialize the length header
	lenBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(lenBytes, uint64(len(msg)))

	// Prefix the message with the length header
	msg = append(lenBytes, msg...)

	// Send to each conn
	s.connLock.RLock()
	conns := make([]net.Conn, len(s.conns))
	copy(conns, s.conns)
	s.connLock.RUnlock()

	var err error
	for _, conn := range conns {
		if _, err = conn.Write(msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Socket) Close() error {
	close(s.quitCh)
	<-s.doneCh

	s.connLock.Lock()
	s.connLock.Unlock()

	conns := s.conns
	s.conns = nil

	var err error
	for _, conn := range conns {
		if err = conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Socket) accept(l net.Listener) error {
	conn, err := l.Accept()
	if err != nil {
		return err
	}
	s.connLock.Lock()
	s.conns = append(s.conns, conn)
	s.connLock.Unlock()
	return nil
}

type Client struct {
	net.Conn
}

func Dial(addr string) (*Client, error) {
	c, err := net.Dial("unix", addr)
	if err != nil {
		return nil, err
	}
	return &Client{c}, nil
}

func (c *Client) Recv(ctx context.Context) ([]byte, error) {
	var sz int64
	var err error

	// deadline, _ := ctx.Deadline()
	c.Conn.SetDeadline(time.Unix(0, 0))
	// c.Conn.SetReadDeadline(time.Unix(0))
	// c.Conn.SetReadDeadline(time.Now().Add(1 * time.Hour))
	if err = binary.Read(c.Conn, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	msg := make([]byte, sz)
	if _, err = io.ReadFull(c.Conn, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Client) Close() error {
	return c.Conn.Close()
}
