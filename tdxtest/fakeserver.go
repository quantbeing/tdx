package tdxtest

import (
	"encoding/binary"
	"net"
	"sync"

	"github.com/quantbeing/tdx/frame"
)

type Server struct {
	Addr string
	ln   net.Listener
	wg   sync.WaitGroup
}

func Start(responses [][]byte) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{Addr: ln.Addr().String(), ln: ln}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		for _, resp := range responses {
			_, _ = conn.Read(buf)
			_, _ = conn.Write(Response(resp))
		}
	}()
	return s, nil
}

func (s *Server) Close() error {
	err := s.ln.Close()
	s.wg.Wait()
	return err
}

func Response(body []byte) []byte {
	header := make([]byte, frame.HeaderSize)
	binary.LittleEndian.PutUint16(header[12:14], uint16(len(body)))
	binary.LittleEndian.PutUint16(header[14:16], uint16(len(body)))
	return append(header, body...)
}
