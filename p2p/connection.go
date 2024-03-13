package p2p

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

var (
	ErrWouldBlock = errors.New("operation would block")
)

// To create a multiplexed stream we will need each packet sent
// through the underlying connection to have some data attached
// to it. This will allow us to identify the packet type, the
// channel ID over which it belongs to, the length in bytes of
// the data contained in the packet, and an EOF to indicate the
// end of the stream. The EOF could be part of the packet type
// field.

type packetType uint8

const (
	packetType_Unspecified byte = iota
	packetType_Message
	packetType_StartStream
	packetType_StreamData
	packetType_EndStream
)

type rawPacketHeader [7]byte

func (p rawPacketHeader) packetType() byte {
	return p[0]
}

func (p rawPacketHeader) payloadLength() uint16 {
	return binary.LittleEndian.Uint16(p[1:])
}

func (p rawPacketHeader) streamId() uint32 {
	return binary.LittleEndian.Uint32(p[3:])
}

type packetHeader struct {
	packetType    byte
	payloadLength uint16
	streamId      uint32
}

func (p *packetHeader) Marshall() [7]byte {
	rawHeader := [7]byte{p.packetType}
	binary.LittleEndian.PutUint16(rawHeader[1:], p.payloadLength)
	binary.LittleEndian.PutUint32(rawHeader[3:], p.streamId)
	return rawHeader
}

func (p rawPacketHeader) Unmarshall() *packetHeader {
	return &packetHeader{
		packetType:    p[0],
		payloadLength: binary.LittleEndian.Uint16(p[1:]),
		streamId:      binary.LittleEndian.Uint32(p[3:]),
	}
}

type packetPayload []byte

// Each Stream will be multiplexed through an MConn.
//
// Stream should implement net.Conn
type Stream struct {
	id   uint32
	conn *MConn

	// Buffer containing packet messages, needs lock
	readBuffer [][]byte
	bMu        sync.Mutex

	// Notifies the stream when there is something to read
	notifyReadCh chan struct{}

	finishOnce       sync.Once
	notifyFinishedCh chan struct{}

	closedCh chan struct{}
}

// Read implements the net.Conn interface
func (s *Stream) Read(b []byte) (int, error) {
	for {
		if len(b) == 0 {
			return 0, nil
		}

		// Read from buffer
		var n int
		s.bMu.Lock()
		if len(s.readBuffer) > 0 {
			n = copy(b, s.readBuffer[0])
			s.readBuffer[0] = s.readBuffer[0][:n]
			if len(s.readBuffer[0]) == 0 {
				s.readBuffer[0] = nil
				s.readBuffer = s.readBuffer[:1]
			}
		}
		s.bMu.Unlock()

		if n > 0 {
			return n, nil
		}

		select {
		case <-s.notifyReadCh:
			continue
		case <-s.notifyFinishedCh:
			s.bMu.Lock()
			if len(s.readBuffer) > 0 {
				s.bMu.Unlock()
				continue
			}
			s.bMu.Unlock()
			return n, io.EOF
		case <-s.closedCh:
			return n, io.ErrClosedPipe
		}

	}
}

func (s *Stream) Write(b []byte) (int, error) {

	select {
	case <-s.closedCh:
		return 0, io.ErrClosedPipe
	default:
	}

	// Write packet message to conn
	data := b
	for len(data) > 0 {
		pMsg := &packetMessage{}

	}

}

func (s *Stream) finished() {
	s.finishOnce.Do(func() {
		s.notifyFinishedCh <- struct{}{}
	})
}

// MConn is a multiplexed connection that will distribute messages
// to/from different channels. It will also support streaming by
// allowing users to open multiplexed streams over the underlying
// connection.
type MConn struct {

	// The underlying net.Conn
	conn io.ReadWriteCloser

	msgReadBuffer  []byte
	msgWritebuffer []byte

	streams map[uint32]Stream
}

func (c *MConn) Send() {

}
