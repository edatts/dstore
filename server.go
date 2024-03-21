package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	// "reflect"
	"sync"

	"github.com/edatts/dstore/p2p"
)

var ErrPeerNotFound = errors.New("peer not found")

func init() {
	// gob.Register(PayloadData_Notify_NewFile{})
	// gob.Register(PayloadData_Request_GetFile{})
	gob.Register(HasFileRequest{})
	gob.Register(HasFileResponse{})
	gob.Register(GetFileRequest{})
	gob.Register(GetFileResponse{})
	gob.Register(PutFileRequest{})
	gob.Register(PutFileResponse{})
	gob.Register(DeleteFileRequest{})
	gob.Register(DeleteFileResponse{})
	gob.Register(PurgeFileRequest{})
	gob.Register(PurgeFileResponse{})
	gob.Register(GetDiskSpaceRequest{})
	gob.Register(GetDiskSpaceResponse{})
}

type ServerOpts struct {
	StorageRoot    string
	Transport      p2p.Transport
	BootstrapNodes []string
}

type Server struct {
	ServerOpts

	pendingOperations map[string]NetworkOperation

	requestedFiles *syncMap[string, chan bool]
	pendingFiles   *set[string]
	peers          *peerMap
	store          *Store
	quitCh         chan struct{}
}

func NewServer(opts ServerOpts) *Server {
	storeOpts := StoreOpts{
		StorageRoot: opts.StorageRoot,
	}

	return &Server{
		ServerOpts: opts,

		requestedFiles: &syncMap[string, chan bool]{
			m:  map[string]chan bool{},
			mu: sync.RWMutex{},
		},
		pendingFiles: newSet[string](),
		peers: &peerMap{
			m:       make(map[net.Addr]p2p.Peer),
			RWMutex: sync.RWMutex{},
		},
		store:  NewStore(storeOpts),
		quitCh: make(chan struct{}),
	}
}

func (s *Server) startBootstrap() error {
	if len(s.BootstrapNodes) == 0 {
		return nil
	}
	for _, addr := range s.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}
		go func(addr string) {
			if err := s.Transport.Dial(addr); err != nil {
				log.Printf("error dialing bootstrap node: %s", err)
			}
		}(addr)
	}

	return nil
}

func (s *Server) Start() error {
	if err := s.Transport.ListenAndAccept(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	go s.startBootstrap()

	s.StartMessageLoop()

	return nil
}

func (s *Server) Stop() {
	s.quitCh <- struct{}{}
}

func (s *Server) StartMessageLoop() {
	for {
		select {
		case msg := <-s.Transport.MsgChan():
			log.Printf("(%s): Received message from: %s", s.Transport.LAddr(), msg.From)

			if err := s.handleMessage(&msg); err != nil {
				log.Printf("(%s): Failed to process message: %s\n", s.Transport.LAddr(), err)
			}

			p, ok := s.peers.Get(msg.From)
			if !ok {
				log.Println("Could not find peer in peers map.")
				continue
			}

			_ = p

		case <-s.quitCh:
			log.Println("Message received on quit channel, stopping server.")
			s.Transport.Close()
			// Should clean up peer conns as well.
			return
		}
	}
}

func (s *Server) channelRequestHandler(c *p2p.Channel) {
	for {
		select {
		case msg := <-c.ConsumeRequests():
			log.Printf("(%s): Received request from: %s", s.Transport.LAddr(), msg.From)

			if err := s.handleRequest(msg); err != nil {
				log.Printf("(%s): Failed to process request: %s\n", s.Transport.LAddr(), err)
			}

		case <-s.quitCh:
			log.Println("Message received on quit channel, stopping server.")
			s.Transport.Close()
			// Should clean up peer conns as well.
			return
		}
	}
}

func (s *Server) handleRequest(msg *p2p.Message) error {

	r := bytes.NewReader(msg.Payload)
	request := RPCRequest{}
	if err := gob.NewDecoder(r).Decode(&request); err != nil {
		return fmt.Errorf("failed to decode payload: %w", err)
	}

	log.Printf("Handling request...")

	switch request.Sum.(type) {
	case *HasFileRequest:
		if err := s.handleHasFileRequest(request.Sum.(*HasFileRequest), msg.From, msg.ChannelId); err != nil {
			return fmt.Errorf("error handling HasFile request: %w", err)
		}
	case *GetFileRequest:
		if err := s.handleGetFileRequest(request.Sum.(*GetFileRequest), msg.From, msg.ChannelId); err != nil {
			return fmt.Errorf("error handling HasFile request: %w", err)
		}
	case *PutFileRequest:
		if err := s.handlePutFileRequest(request.Sum.(*PutFileRequest), msg.From, msg.ChannelId); err != nil {
			return fmt.Errorf("error handling HasFile request: %w", err)
		}
	case *DeleteFileRequest:
		if err := s.handleDeleteFileRequest(request.Sum.(*DeleteFileRequest), msg.From, msg.ChannelId); err != nil {
			return fmt.Errorf("error handling HasFile request: %w", err)
		}
	case *PurgeFileRequest:
		panic("not implemented yet...")
		// if err := s.handlePurgeFileRequest(request.Sum.(*PurgeFileRequest), msg.From, msg.ChannelId); err != nil {
		// 	return fmt.Errorf("error handling HasFile request: %w", err)
		// }
	case *GetDiskSpaceRequest:
		if err := s.handleGetDiskSpaceRequest(request.Sum.(*GetDiskSpaceRequest), msg.From, msg.ChannelId); err != nil {
			return fmt.Errorf("error handling HasFile request: %w", err)
		}
	default:
		return fmt.Errorf("request type not implemented")
	}

	return nil
}

func (s *Server) prepareAndSendResponse(res RPCResponse, addr net.Addr, chId uint32) error {
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(res); err != nil {
		return fmt.Errorf("failed to encode notify payload: %w", err)
	}

	peer, ok := s.peers.Get(addr)
	if !ok {
		return ErrPeerNotFound
	}

	channel, ok := peer.GetChannel(chId)
	if !ok {
		return fmt.Errorf("channel (%d) not found.", chId)
	}

	if err := channel.SendResponse(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to send response on channel (%d)", chId)
	}

	return nil
}

func (s *Server) handleHasFileRequest(req *HasFileRequest, from net.Addr, chId uint32) error {
	var res RPCResponse

	if !s.HasFile(req.FileHash) {
		res = RPCResponse{
			Sum: &HasFileResponse{
				Result: false,
				Err:    nil,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	res = RPCResponse{
		Sum: &HasFileResponse{
			Result: true,
			Err:    nil,
		},
	}

	return s.prepareAndSendResponse(res, from, chId)
}

func (s *Server) handleGetFileRequest(req *GetFileRequest, from net.Addr, chId uint32) error {
	var res RPCResponse

	if !s.HasFile(req.FileHash) {
		res = RPCResponse{
			Sum: &GetFileResponse{
				Result: 0,
				Err:    ErrFileNotFound,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	fileSize, err := s.store.GetFileSize(req.FileHash)
	if err != nil {
		res = RPCResponse{
			Sum: &GetFileResponse{
				Result: 0,
				Err:    ErrInternal,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	res = RPCResponse{
		Sum: &GetFileResponse{
			Result: int(fileSize),
			Err:    nil,
		},
	}

	if err := s.prepareAndSendResponse(res, from, chId); err != nil {
		return fmt.Errorf("failed to respond to request from (%s): %w", from, err)
	}

	// Should probably make this it's own helper func.
	go func() {
		// After sending positive response we can start streaming the data.
		f, err := s.store.Read(req.FileHash)
		if err != nil {
			log.Printf("error: failed to open file (%s)", req.FileHash)
		}

		defer f.Close()

		peer, ok := s.peers.Get(from)
		if !ok {
			log.Printf("failed to get peer (%s)", from)
			return
		}

		channel, ok := peer.GetChannel(chId)
		if !ok {
			log.Printf("failed to get channel (%s)", chId)
			return
		}

		_, err = io.Copy(channel, f)
		if err != nil {
			log.Printf("error: failed to copy: %s", err)
		}

	}()

	return nil
}

func (s *Server) handlePutFileRequest(req *PutFileRequest, from net.Addr, chId uint32) error {
	var res RPCResponse

	if s.HasFile(req.FileHash) {
		res = RPCResponse{
			Sum: &PutFileResponse{
				Result: false,
				Err:    ErrHaveFile,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	// Check our available disk space
	nBytes, err := s.store.GetAvailableDiskBytes()
	if err != nil {
		res = RPCResponse{
			Sum: &PutFileResponse{
				Result: false,
				Err:    ErrInternal,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	if req.FileSize > nBytes {
		res = RPCResponse{
			Sum: &PutFileResponse{
				Result: false,
				Err:    ErrDiskSpace,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	res = RPCResponse{
		Sum: &PutFileResponse{
			Result: true,
			Err:    nil,
		},
	}

	if err := s.prepareAndSendResponse(res, from, chId); err != nil {
		return fmt.Errorf("failed to send response to (%s): %w", from, err)
	}

	// If sending successful response we can start receiving the
	// file from stream.
	go func() {

		peer, ok := s.peers.Get(from)
		if !ok {
			log.Printf("error: failed to get peer when recieving file.")
			return
		}

		channel, ok := peer.GetChannel(chId)
		if !ok {
			log.Printf("error: failed to get channel when recieving file.")
			return
		}

		pr, pw := io.Pipe()

		go func() {
			_, err := io.CopyN(pw, channel, int64(req.FileSize))
			if err != nil {
				log.Printf("error: failed copying file from peer")
			}
		}()

		fileHash, err := s.store.Write(pr)
		if err != nil {
			log.Printf("error: failed writing to store: %w", err)
		}

		if fileHash != req.FileHash {
			panic(fmt.Sprintf("wrong file hash after storage, expected (%s), got (%s)", req.FileHash, fileHash))
		}

	}()

	return nil
}

func (s *Server) handleDeleteFileRequest(req *DeleteFileRequest, from net.Addr, chId uint32) error {
	var res = RPCResponse{
		Sum: &DeleteFileResponse{
			Result: true,
			Err:    nil,
		},
	}

	if !s.HasFile(req.FileHash) {
		return s.prepareAndSendResponse(res, from, chId)
	}

	if err := s.store.Delete(req.FileHash); err != nil {
		log.Printf("failed deleting file: %w", err)
		res = RPCResponse{
			Sum: &DeleteFileResponse{
				Result: false,
				Err:    ErrInternal,
			},
		}
		return s.prepareAndSendResponse(res, from, chId)
	}

	return s.prepareAndSendResponse(res, from, chId)
}

func (s *Server) handlePurgeFileRequest(req *PurgeFileRequest, from net.Addr, chId uint32) error {

	return nil
}

func (s *Server) handleGetDiskSpaceRequest(req *GetDiskSpaceRequest, from net.Addr, chId uint32) error {
	var res RPCResponse

	n, err := s.store.GetAvailableDiskBytes()
	log.Printf("error: failed getting available disk space: %w", err)
	if err != nil {
		res = RPCResponse{
			Sum: &GetDiskSpaceResponse{
				Result: n,
				Err:    ErrInternal,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	res = RPCResponse{
		Sum: &GetDiskSpaceResponse{
			Result: n,
			Err:    nil,
		},
	}

	return s.prepareAndSendResponse(res, from, chId)
}

func (s *Server) prepareAndSendRequest(req RPCRequest, addr net.Addr, chId uint32) error {
	var buf = new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	peer, ok := s.peers.Get(addr)
	if !ok {
		return fmt.Errorf("failed to get peer (%s)", addr)
	}

	channel, ok := peer.GetChannel(chId)
	if !ok {
		return fmt.Errorf("failed getting channel (%s)", chId)
	}

	if err := channel.SendRequest(buf.Bytes()); err != nil {
		return fmt.Errorf("failed sending request on channel: %w", err)
	}

	return nil
}

// Stream data to peers that don't already have the file.
// func (s *Server) BroadcastData(r io.ReadCloser) error {
// 	peers := []io.Writer{}
// 	for _, peer := range s.peers.Values() {
// 		peers = append(peers, peer)
// 	}

// 	// We shouldn't really use a multiwriter because if we fail
// 	// to write to one peer then the stream will end to all peers.
// 	mw := io.MultiWriter(peers...)

// 	_, err := io.Copy(mw, bytes.NewReader([]byte{p2p.StartStream}))
// 	if err != nil {
// 		return fmt.Errorf("failed to copy data to peers: %w", err)
// 	}

// 	_, err = io.Copy(mw, r)
// 	if err != nil {
// 		return fmt.Errorf("failed to copy data to peers: %w", err)
// 	}

// 	if err = r.Close(); err != nil {
// 		return fmt.Errorf("failed closing read closer: %w", err)
// 	}

// 	return nil
// }

func (s *Server) BroadcastMessage(data []byte) {
	for _, peer := range s.peers.Values() {
		log.Printf("[%s]: Broadcasting message to peer (%s)", s.Transport.LAddr(), peer.RemoteAddr())
		if err := peer.SendMessage(data); err != nil {
			log.Printf("Failed sending payload to peer (%s): %s", peer.RemoteAddr(), err)
		}
	}
}

func (s *Server) handleMessage(msg *p2p.Message) error {
	// The issue with handling messages in this way is that a lot of the
	// operations and exchanges between peers that we want to carry out
	// require multiple messages to be sent between peers that need to be
	// processed in a particular order. This function only really works for
	// one off messages because there is no way to prevent race conditions
	// using this central handler.
	//
	// If we need messages to be processed sequentially in a particular
	// order then we need a more robust set of handling logic to ensure
	// that we prevent race conditions.
	//
	// One way we could do this is by multiplexing our connection into
	// multiple channels such that each operation will only receive
	// messages that it would expect to recieve as part of that operation.
	// This would probably be the most performat method but would be
	// complex to implement.
	//
	// Alternatively we could simply queue all the messages we receive
	// for each peer and then loop through them, re-queueing the ones that
	// we wouldn't expect to handle as part of the current operation. This
	// would be less performant and would require timeouts on each operation
	//

	r := bytes.NewReader(msg.Payload)
	payload := Payload{}
	if err := gob.NewDecoder(r).Decode(&payload); err != nil {
		return fmt.Errorf("failed to decode payload: %w", err)
	}

	log.Printf("Handling message. Payload Type: %s", payload.Type)

	// log.Printf("Payload Data: %v", payload.Data)
	// log.Printf("Payload Data Type: %v", reflect.TypeOf(payload.Data))

	switch payload.Type {
	case PayloadType_Notify_NewFile:
		peer, ok := s.peers.Get(msg.From)
		if !ok {
			return fmt.Errorf("peer (%s) not found", msg.From)
		}

		// Get file hash and metadata
		data := payload.Data

		fileHash := data.FileHash

		// Check if we have file.
		if s.HasFile(fileHash) {
			log.Printf("(%s): File already present on disk.", s.Transport.LAddr())
			return nil
		}

		// If we are already writing the file, return
		if s.pendingFiles.Has(fileHash) {
			log.Printf("(%s): File already pending.", s.Transport.LAddr())
			return nil
		}

		s.pendingFiles.Add(fileHash)
		defer s.pendingFiles.Remove(fileHash)

		// Request the file from the peer that notified us
		stream, err := peer.StartStream()
		if err != nil {
			return fmt.Errorf("error opening stream to peer (%s): %w", peer.RemoteAddr(), err)
		}

		defer stream.Close()

		buf := new(bytes.Buffer)
		payload := Payload{
			Type: PayloadType_Request_GetFile,
			Data: PayloadData{
				FileHash: fileHash,
				StreamId: stream.Id(),
			},
		}

		if err := gob.NewEncoder(buf).Encode(payload); err != nil {
			return fmt.Errorf("failed to encode request payload: %w", err)
		}

		peer.SendMessage(buf.Bytes())

		hash, err := s.StoreFile(stream)
		if err != nil {
			// We might need to clean up tmp/ here as well.
			return fmt.Errorf("failed to store file: %w", err)
		}

		log.Printf("(%s): Retrieved and stored file (%s).", s.Transport.LAddr(), hash)

		if hash != fileHash {
			panic(fmt.Sprintf("wrong file hash after write, got %s, expected %s", hash, fileHash))
			// return fmt.Errorf("wrong file hash after write, got %s, expected %s", hash, fileHash)
		}

	case PayloadType_Request_GetFile:
		peer, ok := s.peers.Get(msg.From)
		if !ok {
			return fmt.Errorf("peer (%s) not found", msg.From)
		}

		stream, ok := peer.GetStream(payload.Data.StreamId)
		if !ok {
			return fmt.Errorf("could not find stream with id (%d)", payload.Data.StreamId)
		}

		defer stream.Close()

		// A file has been requested by a peer. For now a peer will only request
		// a file if we have notified them that we have it so we don't need to
		// check for the file before streaming it to them (unless it has been
		// deleted since we notified them...)
		data := payload.Data

		// Check for file
		fileHash := data.FileHash

		if !s.HasFile(fileHash) {
			return fmt.Errorf("file (%s) not found", fileHash)
		}

		f, err := s.store.Read(fileHash)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// fileSize, err := s.store.GetFileSize(fileHash)
		// if err != nil {
		// 	return fmt.Errorf("failed to get file size: %s", err)
		// }

		_, err = io.Copy(stream, f)
		if err != nil {
			// TODO:  If we fail streaming to peer, drop them.
			return fmt.Errorf("failed streaming file to peer (%s): %w", msg.From, err)
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("failed to close reader: %w", err)
		}

	case PayloadType_Notify_HasFile:
		// Check if file has been requested
		fileHash := payload.Data.FileHash
		if !s.requestedFiles.Has(fileHash) || s.pendingFiles.Has(fileHash) {
			return nil
		}

		s.pendingFiles.Add(fileHash)
		defer s.pendingFiles.Remove(fileHash)

		peer, ok := s.peers.Get(msg.From)
		if !ok {
			return ErrPeerNotFound
		}

		// Open stream
		stream, err := peer.StartStream()
		if err != nil {
			return fmt.Errorf("error opening stream to peer (%s): %w", peer.RemoteAddr(), err)
		}

		buf := new(bytes.Buffer)
		payload := Payload{
			Type: PayloadType_Request_GetFile,
			Data: PayloadData{
				FileHash: fileHash,
				StreamId: stream.Id(),
			},
		}

		if err := gob.NewEncoder(buf).Encode(payload); err != nil {
			return fmt.Errorf("failed to encode request payload: %w", err)
		}

		if err := peer.SendMessage(buf.Bytes()); err != nil {
			return fmt.Errorf("failed to send message to peer: %w", err)
		}

		fHash, err := s.store.Write(stream)
		if err != nil {
			return fmt.Errorf("failed writing data from stream to file: %w", err)
		}

		log.Printf("(%s): Retrieved and stored file (%s).", s.Transport.LAddr(), fHash)

		if fHash != fileHash {
			panic(fmt.Sprintf("wrong file hash after write, got %s, expected %s", fHash, fileHash))
			// return fmt.Errorf("wrong file hash after write, got %s, expected %s", hash, fileHash)
		}

		successCh, ok := s.requestedFiles.Get(fileHash)
		if !ok {
			return fmt.Errorf("failed to find result chan for requested file (%s)", fileHash)
		}

		successCh <- true

	case PayloadType_Query_HasFile:
		// Check if we have file. If we do, then we send the file
		// and the metadata.

		fileHash := payload.Data.FileHash

		if !s.HasFile(fileHash) {
			// Should send peer notificaiton that we don't have
			// the requested file. For now just return
			return nil
		}

		peer, ok := s.peers.Get(msg.From)
		if !ok {
			return ErrPeerNotFound
		}

		buf := new(bytes.Buffer)
		payload := Payload{
			Type: PayloadType_Notify_HasFile,
			Data: PayloadData{
				FileHash: fileHash,
			},
		}

		if err := gob.NewEncoder(buf).Encode(payload); err != nil {
			return fmt.Errorf("failed to encode notify payload: %w", err)
		}

		peer.SendMessage(buf.Bytes())

	}

	return nil
}

func (s *Server) HasFile(fileHash string) bool {
	return s.store.fileExists(fileHash)
}

func (s *Server) GetFile(fileHash string) (io.ReadCloser, error) {
	if s.HasFile(fileHash) {
		return s.store.Read(fileHash)
	}

	log.Printf("(%s): File (%s) not found on disk, requesting file from peers...", s.Transport.LAddr(), fileHash)

	// Workflow for this:
	//	- Add file hash to requested files
	// 	- Query all peers for file.
	//	- Request file from first peer to respond.

	successCh := make(chan bool)
	s.requestedFiles.Set(fileHash, successCh)

	s.QueryNetworkHasFile(fileHash)

	// We need to recieve a result here so we know when we can read the file.
	if <-successCh {
		return s.store.Read(fileHash)
	}

	return nil, errors.New("failed to recieve file from network")
}

// Stores a file and returns the file hash. Gossips the
// file content to other nodes for replicated storage.
func (s *Server) StoreFile(r io.Reader) (string, error) {

	// Store file to disk.
	fileHash, err := s.store.Write(r)
	if err != nil {
		return "", fmt.Errorf("failed to write file to disk: %w", err)
	}

	fileSize, err := s.store.GetFileSize(fileHash)
	if err != nil {
		return fileHash, fmt.Errorf("failed to get size of file: %w", err)
	}

	for _, peer := range s.peers.Values() {
		go func(fileHash string) {
			channel, err := peer.OpenChannel()
			if err != nil {
				log.Printf("failed to open channel with peer (%s): ", peer.RemoteAddr(), err)
				return
			}

			req := RPCRequest{
				Sum: &HasFileRequest{
					FileHash: fileHash,
				},
			}

			if err := s.prepareAndSendRequest(req, peer.RemoteAddr(), channel.Id()); err != nil {
				log.Printf("error: failed to send request: %w", err)
				return
			}

			channel.RecvResponse()

		}(fileHash)
	}

	buf := new(bytes.Buffer)
	payload := Payload{
		Type: PayloadType_Notify_NewFile,
		Data: PayloadData{
			FileHash: fileHash,
			Metadata: Metadata{
				FileSize: fileSize,
			},
		},
	}

	err = gob.NewEncoder(buf).Encode(payload)
	if err != nil {
		return fileHash, fmt.Errorf("failed to encode file hash to payload data: %w", err)
	}

	s.BroadcastMessage(buf.Bytes())

	// f, err := s.store.Read(fileHash)
	// if err != nil {
	// 	return fileHash, fmt.Errorf("failed to read file: %w", err)
	// }

	// err = s.BroadcastData(f)
	// if err != nil {
	// 	return fileHash, fmt.Errorf("error broadcasting data: %w", err)
	// }

	// f.Close()

	return fileHash, nil
}

// Deletes file locally.
func (s *Server) DeleteFile(fileHash string) error {
	if !s.HasFile(fileHash) {
		return nil
	}

	if err := s.store.Delete(fileHash); err != nil {
		return fmt.Errorf("error deleting file: %w", err)
	}

	return nil
}

// Deletes file locally and signals to peers that they
// can delete the file as well.
func (s *Server) PurgeFile(fileHash string) error {

	if err := s.DeleteFile(fileHash); err != nil {
		return fmt.Errorf("failed to delete file (%s): %w", fileHash, err)
	}

	// Notify peers to delete file
	buf := new(bytes.Buffer)
	payload := Payload{
		Type: PayloadType_Request_DeleteFile,
		Data: PayloadData{
			FileHash: fileHash,
		},
	}
	if err := gob.NewEncoder(buf).Encode(payload); err != nil {
		return fmt.Errorf("failed to encode deleteFile payload: %w", err)
	}

	s.BroadcastMessage(buf.Bytes())

	// We could mark the file as purge pending and we could
	// periodically check to see if the purge is complete. This
	// would need to be done asynchronously.

	return nil
}

func (s *Server) QueryNetworkHasFile(fileHash string) error {
	buf := new(bytes.Buffer)
	payload := Payload{
		Type: PayloadType_Query_HasFile,
		Data: PayloadData{
			FileHash: fileHash,
		},
	}

	if err := gob.NewEncoder(buf).Encode(payload); err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	s.BroadcastMessage(buf.Bytes())

	return nil
}

func (s *Server) OnPeer(peer p2p.Peer) error {
	s.addPeer(peer)
	return nil
}

func (s *Server) addPeer(peer p2p.Peer) error {
	err := s.peers.Set(peer, peer.RemoteAddr())
	if err != nil {
		return fmt.Errorf("peer already present")
	}
	log.Printf("(%s): Added peer with remote address: %s", s.Transport.LAddr(), peer.RemoteAddr())
	return nil
}

func newSet[T comparable]() *set[T] {
	return &set[T]{
		m:  map[T]struct{}{},
		mu: sync.RWMutex{},
	}
}

type set[T comparable] struct {
	m  map[T]struct{}
	mu sync.RWMutex
}

func (s *set[T]) Add(t T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[t] = struct{}{}
}

func (s *set[T]) Remove(t T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, t)
}

func (s *set[T]) Has(t T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.m[t]; !ok {
		return false
	}

	return true
}

type peerMap struct {
	m map[net.Addr]p2p.Peer
	sync.RWMutex
}

func (p *peerMap) Set(peer p2p.Peer, addr net.Addr) error {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.m[addr]; ok {
		return fmt.Errorf("peer with addr (%s) already present", addr.String())
	}

	p.m[addr] = peer

	return nil
}

func (p *peerMap) Get(addr net.Addr) (peer p2p.Peer, ok bool) {
	p.RLock()
	defer p.RUnlock()

	peer, ok = p.m[addr]
	return peer, ok
}

func (p *peerMap) Values() []p2p.Peer {
	p.RLock()
	defer p.RUnlock()

	values := []p2p.Peer{}
	for _, val := range p.m {
		values = append(values, val)
	}

	return values
}

type syncMap[K comparable, V any] struct {
	m  map[K]V
	mu sync.RWMutex
}

func (s *syncMap[K, V]) Set(k K, v V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
}

func (s *syncMap[K, V]) Get(k K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *syncMap[K, V]) Delete(k K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
}

func (s *syncMap[K, V]) Has(k K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[k]
	return ok
}
