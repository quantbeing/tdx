package tdxtest

import (
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/quantbeing/tdx/frame"
)

type Server struct {
	Addr string
	ln   net.Listener
	wg   sync.WaitGroup
}

func Start(responses [][]byte) (*Server, error) {
	actions := make([]Action, 0, len(responses))
	for _, response := range responses {
		actions = append(actions, ReadAndRespond(response))
	}
	return StartScript(Script{Connections: []ConnectionScript{{Actions: actions}}})
}

type Script struct {
	Connections []ConnectionScript
}

type ConnectionScript struct {
	Actions []Action
}

type Action func(net.Conn) bool

func StartScript(script Script) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{Addr: ln.Addr().String(), ln: ln}
	if len(script.Connections) == 0 {
		script.Connections = []ConnectionScript{{}}
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for _, connection := range script.Connections {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				for _, action := range connection.Actions {
					if action(conn) {
						return
					}
				}
			}()
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

func ReadAndRespond(body []byte) Action {
	return func(conn net.Conn) bool {
		if !readRequest(conn) {
			return true
		}
		_, _ = conn.Write(Response(body))
		return false
	}
}

func ReadAndRaw(raw []byte) Action {
	return func(conn net.Conn) bool {
		if !readRequest(conn) {
			return true
		}
		_, _ = conn.Write(raw)
		return false
	}
}

func ReadAndBadZlib(raw []byte, unzipSize int) Action {
	return func(conn net.Conn) bool {
		if !readRequest(conn) {
			return true
		}
		_, _ = conn.Write(Frame(raw, unzipSize))
		return false
	}
}

func ReadAndPartialFrame(partialBody []byte, declaredZipSize int) Action {
	return func(conn net.Conn) bool {
		if !readRequest(conn) {
			return true
		}
		header := make([]byte, frame.HeaderSize)
		binary.LittleEndian.PutUint16(header[12:14], uint16(declaredZipSize))
		binary.LittleEndian.PutUint16(header[14:16], uint16(declaredZipSize))
		_, _ = conn.Write(append(header, partialBody...))
		return true
	}
}

func ReadAndClose() Action {
	return func(conn net.Conn) bool {
		_ = readRequest(conn)
		return true
	}
}

func ReadAndDelay(delay time.Duration) Action {
	return func(net.Conn) bool {
		time.Sleep(delay)
		return false
	}
}

func Frame(raw []byte, unzipSize int) []byte {
	header := make([]byte, frame.HeaderSize)
	binary.LittleEndian.PutUint16(header[12:14], uint16(len(raw)))
	binary.LittleEndian.PutUint16(header[14:16], uint16(unzipSize))
	return append(header, raw...)
}

func readRequest(conn net.Conn) bool {
	buf := make([]byte, 4096)
	_, err := conn.Read(buf)
	return err == nil
}
