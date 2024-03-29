package p2p

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func MakePeers() (outMConn, inMConn *TCPMConn, err error) {

	// peerOutLAddr := ":5010"
	peerInLAddr := ":5011"
	inConnCh := make(chan net.Conn)
	isAcceptErr := false
	acceptErr := atomic.Value{}

	ln, err := net.Listen("tcp", peerInLAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen : %w", err)
	}

	go func() {
		inConn, err := ln.Accept()
		if err != nil {
			isAcceptErr = true
			acceptErr.Store(err)
		}
		inConnCh <- inConn
	}()

	outConn, err := net.Dial("tcp", peerInLAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial peer: %s", err)
	}

	if isAcceptErr {
		err = acceptErr.Load().(error)
		return nil, nil, fmt.Errorf("failed to accept conn: %w", err)
	}

	inConn := <-inConnCh

	// Got the conns, upgrade them
	outMConn = NewTCPMConn(outConn, true)
	inMConn = NewTCPMConn(inConn, false)

	return outMConn, inMConn, nil
}

func OpenNChannels(outMConn, inMConn *TCPMConn, n int) (outChannels, inChannels []*Channel, err error) {

	// outStreams, inStreams = []*Stream{}, []*Stream{}

	for i := 0; i < n; i++ {
		stream, err := inMConn.OpenChannel()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to start new stream: %w", err)
		}

		inChannels = append(inChannels, stream)

		// Allow time for streams to open
		time.Sleep(time.Millisecond * 10)

		s, ok := outMConn.channels[stream.id]
		if !ok {
			return nil, nil, fmt.Errorf("could not find stream with id (%d)", stream.id)
		}

		outChannels = append(outChannels, s)
	}

	return outChannels, inChannels, nil
}

func StreamNBytes(outChannels, inChannels []*Channel, n int) (int, error) {
	finishedReads := make(chan *bytes.Buffer, n)
	done := make(chan struct{})
	numReadErrs := 0
	numWriteErrs := 0

	// Start reading
	go func() {
		wg := &sync.WaitGroup{}
		for _, channel := range inChannels {
			wg.Add(1)
			go func(c *Channel, wg *sync.WaitGroup) {
				rBuf := new(bytes.Buffer)
				if _, err := rBuf.ReadFrom(c); err != nil {
					numReadErrs += 1
					log.Printf("error reading from channel (%d): %s", c.id, err)
				}
				// log.Printf("rBuf length: %d", rBuf.Len())
				finishedReads <- rBuf
				wg.Done()
			}(channel, wg)
		}
		wg.Wait()
		done <- struct{}{}
		close(finishedReads)
	}()

	// Wait a bit for reads to start
	time.Sleep(time.Millisecond * 10)

	// Write some data to all streams
	b := make([]byte, n)
	if _, err := rand.Reader.Read(b); err != nil {
		return 0, fmt.Errorf("failed to fill buf with random bytes")
	}

	var nWritten int

	for _, s := range outChannels {
		go func(s *Channel) {
			buf := bytes.NewBuffer(b)
			written, err := io.Copy(s, buf)
			if err != nil {
				numWriteErrs += 1
				log.Printf("failed copying data: %s", err)
			}
			nWritten += int(written)
			err = s.Close()
			if err != nil {
				log.Printf("error closing stream: %s", err)
			}
		}(s)
	}

	<-done

	if numReadErrs >= 1 {
		return nWritten, fmt.Errorf("one or more read errors occured")
	}

	if numWriteErrs >= 1 {
		return nWritten, fmt.Errorf("one or more write errors occured")
	}

	for rBuf := range finishedReads {
		if string(b) != rBuf.String() {
			return nWritten, fmt.Errorf("Read buffer different from written buffer...")
		}
	}

	return nWritten, nil
}

func StreamNBytesWithMultiWriter(outChannels, inChannels []*Channel, n int) (int, error) {
	done := make(chan struct{})
	numReadErrs := 0

	// Start reading
	go func() {
		wg := &sync.WaitGroup{}
		for _, channel := range inChannels {
			wg.Add(1)
			go func(c *Channel, wg *sync.WaitGroup) {
				rBuf := new(bytes.Buffer)
				if _, err := rBuf.ReadFrom(c); err != nil {
					numReadErrs += 1
					log.Printf("error reading from stream (%d): %s", c.id, err)
				}
				log.Printf("rBuf length: %d", rBuf.Len())
				wg.Done()
			}(channel, wg)
		}
		wg.Wait()
		done <- struct{}{}
	}()

	// Wait a bit for reads to start
	time.Sleep(time.Millisecond * 10)

	// Write some data to all streams
	b := make([]byte, n)
	if _, err := rand.Reader.Read(b); err != nil {
		return 0, fmt.Errorf("failed to fill buf with random bytes")
	}

	buf := bytes.NewBuffer(b)
	mws := []io.Writer{}
	for _, s := range outChannels {
		mws = append(mws, s)
	}
	mw := io.MultiWriter(mws...)
	written, err := io.Copy(mw, buf)
	if err != nil {
		return int(written) * len(outChannels), fmt.Errorf("failed copying data: %w", err)
	}

	for _, s := range outChannels {
		err = s.Close()
		if err != nil {
			log.Printf("error closing channel: %s", err)
		}
	}

	<-done

	if numReadErrs >= 1 {
		return int(written) * len(outChannels), fmt.Errorf("one or more read errors occured")
	}

	return int(written) * len(outChannels), nil
}

func TestStreaming(t *testing.T) {

	outMConn, inMConn, err := MakePeers()
	if err != nil {
		t.Errorf("failed to make peer conns: %s", err)
	}

	// Test open stream
	outChannel, err := outMConn.OpenChannel()
	if err != nil {
		t.Errorf("failed to start stream on outMConn: %s", err)
	}

	// Allow time for stream to open
	time.Sleep(time.Millisecond * 10)

	// log.Printf("inStreams: (%d), ouStreams: (%d)", len(inMConn.streams), len(outMConn.streams))

	// Verify that stream exists in both sides of MConn
	if len(outMConn.channels) == 0 {
		t.Errorf("stream not found in outMConn")
	}
	if len(inMConn.channels) == 0 {
		t.Errorf("stream not found in inMConn")
	}

	// Verify channelIds are the same
	inChannel, ok := inMConn.channels[outChannel.id]
	if !ok {
		t.Errorf("could not find stream with id: %d", outChannel.id)
	}

	// Start reading
	done := make(chan struct{})
	readBuf := new(bytes.Buffer)
	go func() {
		log.Printf("Attempting to read from inStream")
		readBuf.ReadFrom(inChannel)
		log.Printf("Finished reading...")
		done <- struct{}{}
	}()

	// Write some data
	b := make([]byte, 1024*1024)
	if _, err := rand.Read(b); err != nil {
		t.Errorf("failed to get random bytes: %s", err)
	}
	buf := bytes.NewBuffer(b)
	buf.WriteTo(outChannel)
	if err := outChannel.Close(); err != nil {
		t.Errorf("error closing stream: %s", err)
	}

	// Wait for reads
	<-done

	log.Printf("Done reading.")

	// Ensure streams are cleaned up
	if len(inMConn.channels) != 0 || len(outMConn.channels) != 0 {
		t.Errorf("not all streams cleaned up")
		log.Printf("inMConn num streams: (%d), outMConn num streams: (%d)", len(inMConn.channels), len(outMConn.channels))
	}

	if len(b) != readBuf.Len() {
		t.Error("wrong number of bytes read")
	}

	log.Printf("expected (%d) bytes, got (%d) bytes", len(b), len(readBuf.Bytes()))

	////// Test multiple concurrent streams //////

	outStreams, inStreams, err := OpenNChannels(inMConn, outMConn, 10)
	if err != nil {
		t.Errorf("error opening streams: %s", err)
	}

	// Check both sides see all streams
	if len(inMConn.channels) != len(outMConn.channels) {
		t.Error("sides have different number of streams")
		log.Printf("inStreams: (%d), outStreams: (%d)", len(inMConn.channels), len(outMConn.channels))
	}

	_, err = StreamNBytes(outStreams, inStreams, 10*1024*1024)
	if err != nil {
		t.Errorf("error streaming bytes: %s", err)
	}

	// Ensure all streams are cleaned up
	if len(inMConn.channels) != 0 || len(outMConn.channels) != 0 {
		t.Errorf("not all streams cleaned up")
		log.Printf("inMConn num streams: (%d), outMConn num streams: (%d)", len(inMConn.channels), len(outMConn.channels))
	}

}

func BenchmarkMultipleStreams(b *testing.B) {

	////// Get MConns //////

	outMConn, inMConn, err := MakePeers()
	if err != nil {
		b.Errorf("failed to get MConns %s", err)
	}

	for i := 0; i < b.N; i++ {

		////// Get Streams //////
		outStreams, inStreams, err := OpenNChannels(outMConn, inMConn, 10)
		if err != nil {
			b.Errorf("failed to open streams: %s", err)
		}

		////// Write N Bytes to streams //////
		_, err = StreamNBytes(outStreams, inStreams, 10*1024*1024)
		if err != nil {
			b.Errorf("error streaming bytes: %s", err)
		}
	}

}

var nRes int

func BenchmarkMultipleStreams2(b *testing.B) {

	////// Get MConns //////

	outMConn, inMConn, err := MakePeers()
	if err != nil {
		b.Errorf("failed to get MConns %s", err)
	}

	var n int

	for i := 0; i < b.N; i++ {

		////// Get Streams //////
		outStreams, inStreams, err := OpenNChannels(outMConn, inMConn, 10)
		if err != nil {
			b.Errorf("failed to open streams: %s", err)
		}

		////// Write N Bytes to streams //////
		_, err = StreamNBytes(outStreams, inStreams, 10*1024*1024)
		if err != nil {
			b.Errorf("error streaming bytes: %s", err)
		}
	}

	nRes = n

}

// Test error handling when closing streams during streaming

// Test closing stream twice (should return io.ErrClosedPipe)

// Test writing to stream after close (expect io.ErrClosedPipe)

// Test error handling when closing MConn during streaming

// Test closing MConn twice (should return io.ErrClosedPipe)

// Test blocking behaviour of writing to stream
