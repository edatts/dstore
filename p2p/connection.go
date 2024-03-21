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

	//// I think we only need to do this if we need to send
	//// really big messages, like for example chunks of
	//// storage metadata. However, we can just utilize
	//// the streams and our own messages external to this
	//// package to handle those cases.
	// packetType_StartMessage
	// packetType_MessageData
	// packetType_EndMessage
	packetType_Message

	packetType_RequestMessage
	packetType_ResponseMessage

	packetType_OpenChannel
	packetType_StreamData
	packetType_CloseChannel

	// packetType_StartStream
	// packetType_StreamData
	// packetType_EndStream
)

const rawHeaderSize = 7

type rawPacketHeader [rawHeaderSize]byte

func (p rawPacketHeader) packetType() byte {
	return p[0]
}

func (p rawPacketHeader) channelId() uint32 {
	return binary.LittleEndian.Uint32(p[1:])
}

func (p rawPacketHeader) payloadSize() uint16 {
	return binary.LittleEndian.Uint16(p[5:])
}

type packetHeader struct {
	packetType  byte
	channelId   uint32
	payloadSize uint16
}

func (p *packetHeader) Marshall() [rawHeaderSize]byte {
	rawHeader := [rawHeaderSize]byte{p.packetType}
	binary.LittleEndian.PutUint32(rawHeader[1:], p.channelId)
	binary.LittleEndian.PutUint16(rawHeader[5:], p.payloadSize)
	return rawHeader
}

func (p rawPacketHeader) Unmarshall() *packetHeader {
	return &packetHeader{
		packetType:  p[0],
		channelId:   binary.LittleEndian.Uint32(p[1:]),
		payloadSize: binary.LittleEndian.Uint16(p[5:]),
	}
}

type packet struct {
	header  packetHeader
	payload []byte
}

// Each Channel will be multiplexed through an MConn.
//
// Channel should implement Reader, Writer, and Closer.
// type Channel interface {
// 	Read(b []byte) (int, error)

// 	Write(b []byte) (int, error)

// 	Close() error
// }

type Channel struct {
	id    uint32
	mconn *TCPMConn

	// Buffer containing packet messages, needs lock
	readBuffer [][]byte
	bMu        sync.Mutex

	requestBuf [][]byte
	requestCh  chan *Message

	responseBuf [][]byte
	responseCh  chan *Message

	// Notifies the Channel when there is something to read
	notifyReadCh chan struct{}

	finishOnce       sync.Once
	notifyFinishedCh chan struct{}

	closeOnce sync.Once
	closedCh  chan struct{}
}

func (c *Channel) Id() uint32 {
	return c.id
}

func (c *Channel) ConsumeRequests() chan *Message {
	return c.requestCh
}

// Read implements the net.Conn interface
func (c *Channel) Read(b []byte) (int, error) {
	for {
		if len(b) == 0 {
			return 0, nil
		}

		// Read from buffer
		var n int
		c.bMu.Lock()
		if len(c.readBuffer) > 0 {
			// log.Printf("Read buffer length: %d", len(c.readBuffer))
			// log.Printf("Read buffer first elem length: %d", len(c.readBuffer[0]))
			n = copy(b, c.readBuffer[0])
			c.readBuffer[0] = c.readBuffer[0][n:]
			if len(c.readBuffer[0]) == 0 {
				c.readBuffer[0] = nil
				c.readBuffer = c.readBuffer[1:]
			}
		}
		c.bMu.Unlock()

		// if n > 0 {
		// 	log.Printf("Read (%d) bytes.", n)
		// }

		if n > 0 {
			return n, nil
		}

		select {
		case <-c.notifyReadCh:
			// log.Printf("Notifying of read...")
			continue
		case <-c.notifyFinishedCh:
			// log.Printf("stream (%d) finished...", c.id)
			c.bMu.Lock()
			if len(c.readBuffer) > 0 {
				c.bMu.Unlock()
				continue
			}
			c.Close()
			c.bMu.Unlock()
			// log.Printf("returning io.EOF")
			return n, io.EOF
		case <-c.mconn.connReadErrCh:
			return n, c.mconn.connReadErrVal.Load().(error)
		case <-c.closedCh:
			log.Printf("[stream %d]: event on closedCh", c.id)
			return n, io.ErrClosedPipe
		}

	}
}

func (c *Channel) Write(b []byte) (int, error) {

	// log.Printf("Writing (%d) bytes to stream", len(b))

	select {
	case <-c.closedCh:
		return 0, io.ErrClosedPipe
	default:
	}

	// Write packet message to conn
	var n int
	data := b
	for len(data) > 0 {
		size := len(data)
		if size > int(c.mconn.packetPayloadSize) {
			size = int(c.mconn.packetPayloadSize)
		}
		p := &packet{
			header: packetHeader{
				packetType:  packetType_StreamData,
				channelId:   c.id,
				payloadSize: uint16(size),
			},
			payload: data[:size],
		}

		data = data[size:]

		// log.Printf("Writing packet with header: %+v", p.header)

		nw, err := c.mconn.writePacket(p)
		n += nw
		if err != nil {
			return n, fmt.Errorf("failed to write packet: %w", err)
		}

	}

	return n, nil
}

func (c *Channel) RecvResponse() []byte {

	for {
		select {
		case <-c.closedCh:
		}
	}

	return nil
}

func (c *Channel) SendRequest(req []byte) error {
	if len(req) > int(c.mconn.packetPayloadSize) {
		return errors.New("message exceeds max packet payload size")
	}

	p := &packet{
		header: packetHeader{
			packetType:  packetType_RequestMessage,
			channelId:   c.id,
			payloadSize: uint16(len(req)),
		},
		payload: req,
	}

	_, err := c.mconn.writePacket(p)
	if err != nil {
		return fmt.Errorf("failed to write packet to underlying conn: %w", err)
	}

	return nil
}

func (c *Channel) SendResponse(res []byte) error {

	if len(res) > int(c.mconn.packetPayloadSize) {
		return errors.New("message exceeds max packet payload size")
	}

	p := &packet{
		header: packetHeader{
			packetType:  packetType_ResponseMessage,
			channelId:   c.id,
			payloadSize: uint16(len(res)),
		},
		payload: res,
	}

	_, err := c.mconn.writePacket(p)
	if err != nil {
		return fmt.Errorf("failed to write packet to underlying conn: %w", err)
	}

	return nil
}

// Wraps a message into a packet for sending. The size of
// the message must not exceed m.packetPayloadSize
func (c *Channel) SendMessage(data []byte) error {

	if len(data) > int(c.mconn.packetPayloadSize) {
		return errors.New("message too big")
	}

	// Wrap in packet
	p := &packet{
		header: packetHeader{
			packetType: packetType_Message,
			channelId:  c.id,
		},
		payload: data,
	}

	_, err := c.mconn.writePacket(p)
	if err != nil {
		return fmt.Errorf("failed to write message packet: %w", err)
	}

	return nil
}

func (c *Channel) Close() error {
	var firstCloseCall bool

	// log.Printf("closing stream (%d)...", c.id)

	select {
	case <-c.notifyFinishedCh:
		// Stream closed from other side, clean up
		c.mconn.mu.Lock()
		delete(c.mconn.channels, c.id)
		c.mconn.mu.Unlock()
		return nil

	default:
		c.closeOnce.Do(func() {
			close(c.closedCh)
			firstCloseCall = true
		})

		if firstCloseCall {
			p := &packet{
				header: packetHeader{
					packetType: packetType_CloseChannel,
					channelId:  c.id,
				},
				payload: []byte{},
			}

			_, err := c.mconn.writePacket(p)
			if err != nil {
				return fmt.Errorf("failed to write end stream packet: %w", err)
			}

			c.mconn.onChannelClose(c.id)
			return nil
		} else {
			return io.ErrClosedPipe
		}
	}

}

func (c *Channel) responseLoop() {
	for {

		// TODO: change this to lock only the buffer needed.
		c.bMu.Lock()
		data := make([]byte, len(c.responseBuf[0]))
		if len(c.responseBuf) > 0 {

		}
		c.bMu.Unlock()

		select {
		case c.responseCh <- data:
		}

	}
}

func (c *Channel) requestLoop() {
	for {

		c.bMu.Lock()
		data := make([]byte, len(c.requestBuf[0]))
		if len(c.requestBuf) > 0 {
			copy(data, c.requestBuf[0])
			c.requestBuf[0] = nil
			c.requestBuf = c.requestBuf[1:]
		}
		c.bMu.Unlock()

		req := &Message{
			From:      c.mconn.RemoteAddr(),
			Payload:   data,
			ChannelId: c.id,
		}

		select {
		case c.requestCh <- req:
			continue
		case <-c.closedCh:
			return
		case <-c.mconn.connReadErrCh:
			return
		}

	}
}

func (c *Channel) newPayload(b []byte) {
	c.bMu.Lock()
	defer c.bMu.Unlock()
	c.readBuffer = append(c.readBuffer, b)
}

func (c *Channel) notifyRead() {
	select {
	case c.notifyReadCh <- struct{}{}:
	default:
	}
}

func (c *Channel) newRequestMessage(data []byte) {
	c.bMu.Lock()
	defer c.bMu.Unlock()
	c.requestBuf = append(c.requestBuf, data)
}

func (c *Channel) newResponseMessage(data []byte) {
	c.bMu.Lock()
	defer c.bMu.Unlock()
	c.responseBuf = append(c.responseBuf, data)

}

func (c *Channel) finished() {
	c.finishOnce.Do(func() {
		c.notifyFinishedCh <- struct{}{}
	})
}

// MConn is a multiplexed connection that will distribute messages
// to/from different channels. It will also support streaming by
// allowing users to open multiplexed streams over the underlying
// connection.

type MConn interface {
	OpenChannel() (*Channel, error)
	GetChannel(uint32) (*Channel, bool)

	// StartStream() (*Channel, error)
	// GetStream(uint32) (*Channel, bool)

	// SendMessage([]byte) error
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

	channels map[uint32]*Channel
	mu       sync.RWMutex

	nextChannelId uint32

	reqHandleFunc func([]byte)

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
	m.channels = make(map[uint32]*Channel)
	m.packetPayloadSize = 32 * 1024
	m.writeCh = make(chan *writeReq)
	m.messageCh = make(chan []byte, 100)

	if isOutbound {
		m.nextChannelId = 0
	} else {
		m.nextChannelId = 1
	}

	go m.readLoop()
	go m.writeLoop()

	return m
}

func (m *TCPMConn) SetReqHandleFunc(f func([]byte)) {
	m.reqHandleFunc = f
}

func (m *TCPMConn) newChannel(sid uint32) *Channel {
	s := new(Channel)
	s.id = sid
	s.mconn = m
	s.requestCh = make(chan *Message)
	s.notifyReadCh = make(chan struct{})
	s.notifyFinishedCh = make(chan struct{})
	s.closedCh = make(chan struct{})

	return s
}

func (m *TCPMConn) OpenChannel() (*Channel, error) {
	// Check if MConn is closed
	if m.IsClosed() {
		return nil, io.EOF
	}

	m.mu.Lock()
	m.nextChannelId += 2
	chId := m.nextChannelId
	// TODO: Handle overflow
	m.mu.Unlock()

	log.Printf("Opening channel with ID: %d", chId)

	channel := m.newChannel(chId)

	p := &packet{
		header: packetHeader{
			packetType: packetType_OpenChannel,
			channelId:  chId,
		},
		payload: []byte{},
	}

	_, err := m.writePacket(p)
	if err != nil {
		return &Channel{}, fmt.Errorf("failed to write start stream packet: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[chId] = channel
	return channel, nil

}

func (m *TCPMConn) GetChannel(id uint32) (*Channel, bool) {
	log.Printf("Channels: %+v", m.channels)
	stream, ok := m.channels[id]
	return stream, ok
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
	binary.LittleEndian.PutUint32(rawPacket[1:], p.header.channelId)
	binary.LittleEndian.PutUint16(rawPacket[5:], p.header.payloadSize)

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
			channelId := hdr.channelId()

			switch hdr.packetType() {
			// We need to split the message case into two parts, one to
			// handle requests and one to handle responses.
			// case packetType_Message:
			// 	buf := make([]byte, hdr.payloadSize())
			// 	_, err := io.ReadFull(m.conn, buf)
			// 	if err == nil || err == io.EOF {
			// 		m.mu.Lock()
			// 		channel, ok := m.channels[channelId]
			// 		if ok {
			// 			channel.newPayload(buf)
			// 			channel.notifyRead()
			// 		}
			// 		m.mu.Unlock()
			// 	} else {
			// 		log.Printf("error reading packet payload: %s", err)
			// 		m.onConnReadError(err)
			// 		return
			// 	}

			case packetType_RequestMessage:
				// RPC requests will get passed straight into the user provided
				// message handler.
				buf := make([]byte, hdr.payloadSize())
				_, err := io.ReadFull(m.conn, buf)
				if err == nil || err == io.EOF {
					m.mu.Lock()
					channel, ok := m.channels[channelId]
					if ok {
						channel.newRequestMessage(buf)
						channel.notifyRequest()

					}
					m.mu.Unlock()
				} else {
					log.Printf("error reading packet payload: %s", err)
					m.onConnReadError(err)
					return
				}

			case packetType_ResponseMessage:
				// RPC responses will get queued on the channel for the user to
				// dequeue and process manually.
				buf := make([]byte, hdr.payloadSize())
				_, err := io.ReadFull(m.conn, buf)
				if err == nil || err == io.EOF {
					m.mu.Lock()
					channel, ok := m.channels[channelId]
					if ok {
						channel.newResponseMessage(buf)
						channel.notifyResponse()
					}
					m.mu.Unlock()
				} else {
					log.Printf("error reading packet payload: %s", err)
					m.onConnReadError(err)
					return
				}

			case packetType_OpenChannel:
				m.mu.Lock()
				if _, ok := m.channels[channelId]; !ok {
					channel := m.newChannel(channelId)
					m.channels[channelId] = channel
					// log.Printf("New incoming channel")

					go channel.requestLoop()

				}
				m.mu.Unlock()

			case packetType_StreamData:
				buf := make([]byte, hdr.payloadSize())
				_, err = io.ReadFull(m.conn, buf)
				if err == nil {
					m.mu.Lock()
					channel, ok := m.channels[channelId]
					if ok {
						channel.newPayload(buf)
						channel.notifyRead()
					}
					m.mu.Unlock()
				} else {
					// Handle error, log for now
					log.Printf("error reading packet payload: %s", err)
					m.onConnReadError(err)
					return
				}

			case packetType_CloseChannel:
				m.mu.Lock()
				if channel, ok := m.channels[channelId]; ok {
					channel.notifyRead()
					channel.finished()
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

// func (m *TCPMConn) onChannelOpen(channel *Channel) {
// 	go m.onChannelOpenFunc(channel)
// }

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

func (m *TCPMConn) onChannelClose(id uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
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
