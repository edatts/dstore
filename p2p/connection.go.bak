package p2p

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
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

const (
	packetType_Unspecified byte = iota
	packetType_Message
	packetType_StartStream
	packetType_StreamData
	packetType_EndStream
)

const rawHeaderSize = 7

type rawPacketHeader [rawHeaderSize]byte

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

func (p *packetHeader) Marshall() [rawHeaderSize]byte {
	rawHeader := [rawHeaderSize]byte{p.packetType}
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

type packet struct {
	header  packetHeader
	payload []byte
}

// Each Stream will be multiplexed through an MConn.
//
// Stream should implement Reader, Writer, and Closer.
// type Stream interface {
// 	Read(b []byte) (int, error)

// 	Write(b []byte) (int, error)

// 	Close() error
// }

type Stream struct {
	id    uint32
	mconn *TCPMConn

	// Buffer containing packet messages, needs lock
	readBuffer [][]byte
	bMu        sync.Mutex

	// Notifies the stream when there is something to read
	notifyReadCh chan struct{}

	finishOnce       sync.Once
	notifyFinishedCh chan struct{}

	closeOnce sync.Once
	closedCh  chan struct{}
}

func (s *Stream) Id() uint32 {
	return s.id
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
			// log.Printf("Read buffer length: %d", len(s.readBuffer))
			// log.Printf("Read buffer first elem length: %d", len(s.readBuffer[0]))
			n = copy(b, s.readBuffer[0])
			s.readBuffer[0] = s.readBuffer[0][n:]
			if len(s.readBuffer[0]) == 0 {
				s.readBuffer[0] = nil
				s.readBuffer = s.readBuffer[1:]
			}
		}
		s.bMu.Unlock()

		if n > 0 {
			log.Printf("Read (%d) bytes.", n)
		}

		if n > 0 {
			return n, nil
		}

		select {
		case <-s.notifyReadCh:
			log.Printf("Notifying of read...")
			continue
		case <-s.notifyFinishedCh:
			log.Printf("stream (%d) finished...", s.id)
			s.bMu.Lock()
			if len(s.readBuffer) > 0 {
				s.bMu.Unlock()
				continue
			}
			s.Close()
			s.bMu.Unlock()
			log.Printf("returning io.EOF")
			return n, io.EOF
		case <-s.mconn.connReadErrCh:
			return n, s.mconn.connReadErrVal.Load().(error)
		case <-s.closedCh:
			log.Printf("[stream %d]: event on closedCh", s.id)
			return n, io.ErrClosedPipe
		}

	}
}

func (s *Stream) Write(b []byte) (int, error) {

	log.Printf("Writing (%d) bytes to stream", len(b))

	select {
	case <-s.closedCh:
		return 0, io.ErrClosedPipe
	default:
	}

	// Write packet message to conn
	var n int
	data := b
	for len(data) > 0 {
		size := len(data)
		if size > int(s.mconn.packetPayloadSize) {
			size = int(s.mconn.packetPayloadSize)
		}
		p := &packet{
			header: packetHeader{
				packetType:    packetType_StreamData,
				streamId:      s.id,
				payloadLength: uint16(size),
			},
			payload: data[:size],
		}

		data = data[size:]

		nw, err := s.mconn.writePacket(p)
		n += nw
		if err != nil {
			return n, fmt.Errorf("failed to write packet: %w", err)
		}

	}

	return n, nil
}

func (s *Stream) Close() error {
	var firstCloseCall bool

	log.Printf("closing stream (%d)...", s.id)

	select {
	case <-s.notifyFinishedCh:
		// Stream closed from other side, clean up
		s.mconn.mu.Lock()
		delete(s.mconn.streams, s.id)
		s.mconn.mu.Unlock()
		return nil

	default:
		s.closeOnce.Do(func() {
			close(s.closedCh)
			firstCloseCall = true
		})

		if firstCloseCall {
			p := &packet{
				header: packetHeader{
					packetType:    packetType_EndStream,
					payloadLength: 0,
					streamId:      s.id,
				},
				payload: []byte{},
			}

			_, err := s.mconn.writePacket(p)
			if err != nil {
				return fmt.Errorf("failed to write end stream packet: %w", err)
			}

			s.mconn.onStreamClose(s.id)
			return nil
		} else {
			return io.ErrClosedPipe
		}
	}

}

func (s *Stream) newPayload(b []byte) {
	s.bMu.Lock()
	defer s.bMu.Unlock()
	s.readBuffer = append(s.readBuffer, b)
}

func (s *Stream) notifyRead() {
	select {
	case s.notifyReadCh <- struct{}{}:
	default:
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

type MConn interface {
	StartStream() (*Stream, error)
	GetStream(uint32) (*Stream, bool)

	SendMessage([]byte) error
	ConsumeMessages() <-chan []byte

	RemoteAddr() net.Addr

	Close() error
	IsClosed() bool
}

type TCPMConn struct {

	// The underlying net.Conn
	conn net.Conn

	closedCh  chan struct{}
	closeOnce sync.Once

	streams map[uint32]*Stream
	mu      sync.RWMutex

	nextStreamId uint32

	packetPayloadSize uint16

	writeCh chan *writeReq

	messageCh chan []byte

	connReadErrCh    chan struct{}
	connWriteErrCh   chan struct{}
	connReadErrOnce  sync.Once
	connWriteErrOnce sync.Once
	connReadErrVal   *atomic.Value
	connWriteErrVal  *atomic.Value
}

func NewTCPMConn(conn net.Conn, isOutbound bool) *TCPMConn {
	m := new(TCPMConn)
	m.conn = conn
	m.closedCh = make(chan struct{})
	m.streams = make(map[uint32]*Stream)
	m.packetPayloadSize = 32 * 1024
	m.writeCh = make(chan *writeReq)
	m.messageCh = make(chan []byte, 100)

	if isOutbound {
		m.nextStreamId = 0
	} else {
		m.nextStreamId = 1
	}

	go m.readLoop()
	go m.writeLoop()

	return m
}

func (m *TCPMConn) newStream(sid uint32) *Stream {
	s := new(Stream)
	s.id = sid
	s.mconn = m
	s.notifyReadCh = make(chan struct{})
	s.notifyFinishedCh = make(chan struct{})
	s.closedCh = make(chan struct{})

	return s
}

func (m *TCPMConn) StartStream() (*Stream, error) {
	// Check if MConn is closed
	if m.IsClosed() {
		return nil, io.EOF
	}

	// Generate the next streamId
	m.mu.Lock()
	m.nextStreamId += 2
	sid := m.nextStreamId
	// TODO: Handle the case where streamId overflows.
	m.mu.Unlock()

	log.Printf("Starting stream with ID: %d", sid)

	stream := m.newStream(sid)

	p := &packet{
		header: packetHeader{
			packetType:    packetType_StartStream,
			payloadLength: 0,
			streamId:      sid,
		},
		payload: []byte{},
	}

	_, err := m.writePacket(p)
	if err != nil {
		return &Stream{}, fmt.Errorf("failed to write start stream packet: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.streams[sid] = stream
	return stream, nil
}

func (m *TCPMConn) GetStream(id uint32) (*Stream, bool) {
	log.Printf("streams: %+v", m.streams)
	stream, ok := m.streams[id]
	return stream, ok
}

// Wraps a message into a packet for sending. The size of
// the message must not exceed m.packetPayloadSize
func (m *TCPMConn) SendMessage(data []byte) error {

	if len(data) > int(m.packetPayloadSize) {
		return errors.New("message too big")
	}

	// Wrap in packet
	p := &packet{
		header: packetHeader{
			packetType:    packetType_Message,
			payloadLength: uint16(len(data)),
		},
		payload: data,
	}

	_, err := m.writePacket(p)
	if err != nil {
		return fmt.Errorf("failed to write message packet: %w", err)
	}

	return nil
}

func (m *TCPMConn) newMessagePayload(buf []byte) {
	m.messageCh <- buf
}

func (m *TCPMConn) ConsumeMessages() <-chan []byte {
	return m.messageCh
}

type writeReq struct {
	rawPacket []byte
	resultCh  chan *writeResult
}

type writeResult struct {
	n   int
	err error
}

func (m *TCPMConn) writePacket(p *packet) (int, error) {
	var rawPacket = make([]byte, rawHeaderSize+len(p.payload))

	rawPacket[0] = p.header.packetType
	binary.LittleEndian.PutUint16(rawPacket[1:], p.header.payloadLength)
	binary.LittleEndian.PutUint32(rawPacket[3:], p.header.streamId)

	copy(rawPacket[rawHeaderSize:], p.payload)

	req := &writeReq{rawPacket, make(chan *writeResult)}

	select {
	case m.writeCh <- req:
	case <-m.connWriteErrCh:
		return 0, m.connWriteErrVal.Load().(error)
	case <-m.closedCh:
		return 0, io.ErrClosedPipe
	}

	select {
	case res := <-req.resultCh:
		return res.n, res.err
	case <-m.connWriteErrCh:
		return 0, m.connWriteErrVal.Load().(error)
	case <-m.closedCh:
		return 0, io.ErrClosedPipe
	}
}

func (m *TCPMConn) writeLoop() {
	// Need to add some error handling with chans
	var n int
	var err error
	var req *writeReq

	for {
		select {
		case <-m.closedCh:
			return
		case req = <-m.writeCh:
			n, err = m.conn.Write(req.rawPacket)
			if err != nil {
				m.onConnWriteError(err)
			}
		}

		n -= rawHeaderSize
		if n < 0 {
			n = 0
		}

		req.resultCh <- &writeResult{
			n:   n,
			err: err,
		}

		close(req.resultCh)

	}

}

func (m *TCPMConn) readLoop() {
	var hdr rawPacketHeader

	for {

		// Read the header first
		_, err := io.ReadFull(m.conn, hdr[:])
		if err == nil {
			streamId := hdr.streamId()

			switch hdr.packetType() {
			case packetType_Message:
				// TODO: Handle messages
				buf := make([]byte, hdr.payloadLength())
				_, err := io.ReadFull(m.conn, buf)
				if err == nil || err == io.EOF {
					m.newMessagePayload(buf)
				} else {
					log.Printf("error reading packet payload: %s", err)
					m.onConnReadError(err)
					return
				}

			case packetType_StartStream:
				m.mu.Lock()
				if _, ok := m.streams[streamId]; !ok {
					stream := m.newStream(streamId)
					m.streams[streamId] = stream
					log.Printf("New incoming stream")
				}
				m.mu.Unlock()
			case packetType_StreamData:
				buf := make([]byte, hdr.payloadLength())
				_, err = io.ReadFull(m.conn, buf)
				if err == nil {
					m.mu.Lock()
					stream, ok := m.streams[streamId]
					if ok {
						stream.newPayload(buf)
						stream.notifyRead()
					}
					m.mu.Unlock()
				} else {
					// Handle error, log for now
					log.Printf("error reading packet payload: %s", err)
					m.onConnReadError(err)
					return
				}
			case packetType_EndStream:
				m.mu.Lock()
				if stream, ok := m.streams[streamId]; ok {
					stream.notifyRead()
					stream.finished()
				}
				m.mu.Unlock()
			}

		} else {
			// Report write error, for now just log it
			log.Printf("error reading header: %s", err)
			m.onConnReadError(err)
			return
		}

	}

}

func (m *TCPMConn) Close() error {
	var firstClose bool

	m.closeOnce.Do(func() {
		firstClose = true
		close(m.closedCh)
	})

	if firstClose {
		// TODO: Close all streams (we might need a new method on *Stream)

		if err := m.conn.Close(); err != nil {
			return fmt.Errorf("failed to close conn: %w", err)
		}
	} else {
		return io.ErrClosedPipe
	}

	return nil
}

func (m *TCPMConn) RemoteAddr() net.Addr {
	return m.conn.RemoteAddr()
}

func (m *TCPMConn) IsClosed() bool {
	select {
	case <-m.closedCh:
		return true
	default:
		return false
	}
}

func (m *TCPMConn) onStreamClose(sid uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streams, sid)
}

func (m *TCPMConn) onConnReadError(err error) {
	m.connReadErrOnce.Do(func() {
		m.connReadErrVal.Store(err)
		close(m.connReadErrCh)
	})
}

func (m *TCPMConn) onConnWriteError(err error) {
	m.connWriteErrOnce.Do(func() {
		m.connWriteErrVal.Store(err)
		close(m.connWriteErrCh)
	})
}
