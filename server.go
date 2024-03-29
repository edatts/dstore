package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"github.com/edatts/dstore/p2p"
)

var ErrPeerNotFound = errors.New("peer not found")

func init() {
	// gob.Register(PayloadData_Notify_NewFile{})
	// gob.Register(PayloadData_Request_GetFile{})
	gob.Register(&HasFileRequest{})
	gob.Register(&HasFileResponse{})
	gob.Register(&GetFileRequest{})
	gob.Register(&GetFileResponse{})
	gob.Register(&PutFileRequest{})
	gob.Register(&PutFileResponse{})
	gob.Register(&DeleteFileRequest{})
	gob.Register(&DeleteFileResponse{})
	gob.Register(&PurgeFileRequest{})
	gob.Register(&PurgeFileResponse{})
	gob.Register(&GetDiskSpaceRequest{})
	gob.Register(&GetDiskSpaceResponse{})
	gob.Register(RPCError{})
}

type ServerOpts struct {
	StorageRoot    string
	Transport      p2p.Transport
	BootstrapNodes []string
}

type Server struct {
	ServerOpts

	incomingFiles *set[string]
	purgingFiles  *set[string]
	peers         *peerMap
	store         *Store

	quitCh chan struct{}
}

func NewServer(opts ServerOpts) *Server {
	storeOpts := StoreOpts{
		StorageRoot: opts.StorageRoot,
	}

	return &Server{
		ServerOpts:    opts,
		incomingFiles: newSet[string](),
		purgingFiles:  newSet[string](),
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

func (s *Server) Init() error {

	if err := os.MkdirAll(s.StorageRoot, fs.ModePerm); err != nil {
		return fmt.Errorf("failed to create storage root: %s", err)
	}

	return nil
}

func (s *Server) Start() error {
	if err := s.Transport.ListenAndAccept(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	go s.startBootstrap()

	// s.StartMessageLoop()

	// log.Printf("Started node...")

	<-s.quitCh

	return nil
}

func (s *Server) Stop() {
	s.quitCh <- struct{}{}
}

func (s *Server) handleRequest(msg *p2p.Message) error {

	r := bytes.NewReader(msg.Payload)
	request := RPCRequest{}
	if err := gob.NewDecoder(r).Decode(&request); err != nil {
		return fmt.Errorf("failed to decode payload: %w", err)
	}

	// log.Printf("Handling %s message.", reflect.TypeOf(request.Sum))

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
		if err := s.handlePurgeFileRequest(request.Sum.(*PurgeFileRequest), msg.From, msg.ChannelId); err != nil {
			return fmt.Errorf("error handling HasFile request: %w", err)
		}
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
		return fmt.Errorf("failed to encode response payload: %w", err)
	}

	peer, ok := s.peers.Get(addr)
	if !ok {
		return ErrPeerNotFound
	}

	channel, ok := peer.GetChannel(chId)
	if !ok {
		return fmt.Errorf("channel (%d) not found", chId)
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
				Err:    NilErr,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	res = RPCResponse{
		Sum: &HasFileResponse{
			Result: true,
			Err:    NilErr,
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
			Err:    NilErr,
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
			log.Printf("failed to get channel (%d)", chId)
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

	// I think the lock needs to be held for the entirety of the duration
	// of the .Has() and .Add() calls instead of just during their
	// individual execution. Otherwise there is still a data race.
	// This version checks if the file is present and then adds it if
	// it is not present, the underlying lock is held at all times.
	if added := s.incomingFiles.AddIfNotHas(req.FileHash); !added {
		// File already incoming
		res = RPCResponse{
			Sum: &PutFileResponse{
				Result: false,
				Err:    ErrAlreadyRecv,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	res = RPCResponse{
		Sum: &PutFileResponse{
			Result: true,
			Err:    NilErr,
		},
	}

	if err := s.prepareAndSendResponse(res, from, chId); err != nil {
		s.incomingFiles.Remove(req.FileHash)
		return fmt.Errorf("failed to send response to (%s): %w", from, err)
	}

	// If sending successful response we can start receiving the
	// file from stream.
	go func() {
		defer s.incomingFiles.Remove(req.FileHash)

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
			defer pw.Close()
			_, err := io.CopyN(pw, channel, int64(req.FileSize))
			if err != nil {
				log.Printf("error: failed copying file from peer")
			}
		}()

		fileHash, err := s.StoreFile(pr, req.Propagate)
		if err != nil {
			log.Printf("error: failed writing to store: %s", err)
			return
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
			Err:    NilErr,
		},
	}

	if !s.HasFile(req.FileHash) {
		return s.prepareAndSendResponse(res, from, chId)
	}

	if err := s.store.Delete(req.FileHash); err != nil {
		log.Printf("failed deleting file: %s", err)
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
	var res = RPCResponse{
		Sum: &PurgeFileResponse{
			Result: true,
			Err:    NilErr,
		},
	}

	if !s.purgingFiles.AddIfNotHas(req.FileHash) {
		// Already purging file
		return s.prepareAndSendResponse(res, from, chId)
	}
	defer s.purgingFiles.Remove(req.FileHash)

	if err := s.DeleteFile(req.FileHash); err != nil {
		log.Printf("failed deleting file (%s): %s", req.FileHash, err)
		res = RPCResponse{
			Sum: &PurgeFileResponse{
				Result: false,
				Err:    ErrInternal,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	if err := s.NetworkPurgeFile(req.FileHash); err != nil {
		log.Printf("one or more nodes failed purging file (%s): %s", req.FileHash, err)
		res = RPCResponse{
			Sum: &PurgeFileResponse{
				Result: false,
				Err:    ErrPurgeFailed,
			},
		}

		return s.prepareAndSendResponse(res, from, chId)
	}

	return s.prepareAndSendResponse(res, from, chId)
}

func (s *Server) handleGetDiskSpaceRequest(req *GetDiskSpaceRequest, from net.Addr, chId uint32) error {
	var res RPCResponse

	n, err := s.store.GetAvailableDiskBytes()
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
			Err:    NilErr,
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
		return fmt.Errorf("failed getting channel (%d)", chId)
	}

	if err := channel.SendRequest(buf.Bytes()); err != nil {
		return fmt.Errorf("failed sending request on channel: %w", err)
	}

	return nil
}

func (s *Server) MakeHasFileRequest(fileHash string, peer p2p.Peer, chId uint32) (*HasFileResponse, error) {

	channel, ok := peer.GetChannel(chId)
	if !ok {
		return &HasFileResponse{}, fmt.Errorf("failed to get channel for peer (%s)", peer.RemoteAddr())
	}

	req := RPCRequest{
		Sum: &HasFileRequest{
			FileHash: fileHash,
		},
	}

	if err := s.prepareAndSendRequest(req, peer.RemoteAddr(), chId); err != nil {
		// log.Printf("error: failed to send request: %w", err)
		return &HasFileResponse{}, fmt.Errorf("failed to send request: %w", err)
	}

	msg, err := channel.RecvResponse()
	if err != nil {
		// log.Printf("error: failed to receive response from (%s):", peer.RemoteAddr())
		return &HasFileResponse{}, fmt.Errorf("failed to receive response from (%s): %w", peer.RemoteAddr(), err)
	}

	// Decode
	r := bytes.NewReader(msg.Payload)
	res := RPCResponse{}
	if err := gob.NewDecoder(r).Decode(&res); err != nil {
		// log.Printf("err: failed decoding hasFile response: %w", err)
		return &HasFileResponse{}, fmt.Errorf("err: failed decoding hasFile response: %w", err)
	}

	hasFileRes, ok := res.Sum.(*HasFileResponse)
	if !ok {
		// log.Printf("error: unexpected response type, wanted (*HasFileResponse)")
		return &HasFileResponse{}, fmt.Errorf("error: unexpected response type, wanted (*HasFileResponse)")
	}

	if hasFileRes.Err.NotNil() {
		// log.Printf("Remte peer (%s) error: %s", peer.RemoteAddr(), err)
		return &HasFileResponse{}, fmt.Errorf("remote peer (%s) error: %s", peer.RemoteAddr(), hasFileRes.Err)
	}

	return hasFileRes, nil
}

func (s *Server) MakePutFileRequest(fileHash string, fileSize int, peer p2p.Peer, chId uint32, propagate bool) (*PutFileResponse, error) {

	channel, ok := peer.GetChannel(chId)
	if !ok {
		return &PutFileResponse{}, fmt.Errorf("failed to get channel for peer (%s)", peer.RemoteAddr())
	}

	req := RPCRequest{
		Sum: &PutFileRequest{
			FileHash:  fileHash,
			FileSize:  fileSize,
			Propagate: propagate,
		},
	}

	if err := s.prepareAndSendRequest(req, peer.RemoteAddr(), chId); err != nil {
		// log.Printf("error: failed sending request to (%s): %s", peer.RemoteAddr(), err)
		return &PutFileResponse{}, fmt.Errorf("failed sending request to (%s): %w", peer.RemoteAddr(), err)
	}

	msg, err := channel.RecvResponse()
	if err != nil {
		// log.Printf("error: failed to recieve repsonse from (%s): %s", peer.RemoteAddr(), err)
		return &PutFileResponse{}, fmt.Errorf("failed to recieve repsonse from (%s): %w", peer.RemoteAddr(), err)
	}

	r := bytes.NewReader(msg.Payload)
	res := RPCResponse{}
	if err := gob.NewDecoder(r).Decode(&res); err != nil {
		// log.Printf("err: failed decoding putFile response: %w", err)
		return &PutFileResponse{}, fmt.Errorf("failed decoding putFile response: %w", err)
	}

	putFileRes, ok := res.Sum.(*PutFileResponse)
	if !ok {
		// log.Printf("error: unexpected response type, wanted (*HasFileResponse)")
		return &PutFileResponse{}, fmt.Errorf("unexpected response type, wanted (*PutFileResponse)")
	}

	if putFileRes.Err.NotNil() {
		// log.Printf("Remote peer (%s) error: %s", peer.RemoteAddr(), err)
		return &PutFileResponse{}, fmt.Errorf("Remote peer (%s) error: %w", peer.RemoteAddr(), putFileRes.Err)
	}

	return putFileRes, nil
}

func (s *Server) MakeGetFileRequest(fileHash string, peer p2p.Peer, chId uint32) (*GetFileResponse, error) {

	channel, ok := peer.GetChannel(chId)
	if !ok {
		return &GetFileResponse{}, fmt.Errorf("failed to get channel for peer (%s)", peer.RemoteAddr())
	}

	req := RPCRequest{
		Sum: &GetFileRequest{
			FileHash: fileHash,
		},
	}

	if err := s.prepareAndSendRequest(req, peer.RemoteAddr(), chId); err != nil {
		return &GetFileResponse{}, fmt.Errorf("failed sending request to peer (%s): %w", peer.RemoteAddr(), err)
	}

	msg, err := channel.RecvResponse()
	if err != nil {
		return &GetFileResponse{}, fmt.Errorf("failed recieving response from (%s): %w", peer.RemoteAddr(), err)
	}

	r := bytes.NewReader(msg.Payload)
	res := RPCResponse{}
	if err := gob.NewDecoder(r).Decode(&res); err != nil {
		return &GetFileResponse{}, fmt.Errorf("failed decoding getFile response from (%s): %w", peer.RemoteAddr(), err)
	}

	getFileRes, ok := res.Sum.(*GetFileResponse)
	if !ok {
		return &GetFileResponse{}, fmt.Errorf("unexpected response type, wanted (*GetFileResponse)")
	}

	if getFileRes.Err.NotNil() {
		return &GetFileResponse{}, fmt.Errorf("remote peer (%s) error: %w", peer.RemoteAddr(), getFileRes.Err)
	}

	return getFileRes, nil
}

func (s *Server) MakeDeleteFileRequest(fileHash string, peer p2p.Peer, chId uint32) (*DeleteFileResponse, error) {

	channel, ok := peer.GetChannel(chId)
	if !ok {
		return &DeleteFileResponse{}, fmt.Errorf("failed to get channel for peer (%s)", peer.RemoteAddr())
	}

	req := RPCRequest{
		Sum: &DeleteFileRequest{
			FileHash: fileHash,
		},
	}

	if err := s.prepareAndSendRequest(req, peer.RemoteAddr(), chId); err != nil {
		return &DeleteFileResponse{}, fmt.Errorf("failed sending request to peer (%s): %w", peer.RemoteAddr(), err)
	}

	msg, err := channel.RecvResponse()
	if err != nil {
		return &DeleteFileResponse{}, fmt.Errorf("failed recieving response from (%s): %w", peer.RemoteAddr(), err)
	}

	r := bytes.NewReader(msg.Payload)
	res := RPCResponse{}
	if err := gob.NewDecoder(r).Decode(&res); err != nil {
		return &DeleteFileResponse{}, fmt.Errorf("failed decoding deleteFile response from (%s): %w", peer.RemoteAddr(), err)
	}

	deleteFileRes, ok := res.Sum.(*DeleteFileResponse)
	if !ok {
		return &DeleteFileResponse{}, fmt.Errorf("unexpected response type, wanted (*DeleteFileResponse)")
	}

	if deleteFileRes.Err.NotNil() {
		return &DeleteFileResponse{}, fmt.Errorf("remote peer (%s) error: %w", peer.RemoteAddr(), deleteFileRes.Err)
	}

	return deleteFileRes, nil
}

// This one is a little different since when purging a file we need to make the
// request to all peers.
func (s *Server) NetworkPurgeFile(fileHash string) error {
	buf := new(bytes.Buffer)
	req := RPCRequest{
		Sum: &PurgeFileRequest{
			FileHash: fileHash,
		},
	}

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		return fmt.Errorf("failed encoding purgeFile request: %w", err)
	}

	// Make request to all peers and keep track of number of failures
	var numFailures = atomic.Uint32{}
	var wg = &sync.WaitGroup{}
	for _, peer := range s.peers.Values() {
		wg.Add(1)
		go func(peer p2p.Peer, wg *sync.WaitGroup) {
			defer wg.Done()
			channel, err := peer.OpenChannel()
			if err != nil {
				log.Printf("failed to open channel with peer (%s): %s", peer.RemoteAddr(), err)
				return
			}

			defer channel.Close()

			if err := channel.SendRequest(buf.Bytes()); err != nil {
				log.Printf("failed sending purgeFile request to peer (%s)", peer.RemoteAddr())
				return
			}

			msg, err := channel.RecvResponse()
			if err != nil {
				log.Printf("failed receiving message from peer (%s): %s", peer.RemoteAddr(), err)
				return
			}

			r := bytes.NewReader(msg.Payload)
			res := RPCResponse{}
			if err := gob.NewDecoder(r).Decode(&res); err != nil {
				log.Printf("failed decoding deleteFile response from peer (%s): %s", peer.RemoteAddr(), err)
				return
			}

			purgeFileRes, ok := res.Sum.(*PurgeFileResponse)
			if !ok {
				log.Printf("unexpected response type, wanted (*PurgeFileResponse)")
				return
			}

			if purgeFileRes.Err.NotNil() || !purgeFileRes.Result {
				// Failed purge
				numFailures.Add(1)
			}

		}(peer, wg)
	}

	wg.Wait()

	if numFailures.Load() > 0 {
		return fmt.Errorf("one or more nodes failed to acknowledge purge of file (%s): %w", fileHash, ErrPurgeFailed)
	}

	log.Printf("File (%s) successfully purged from the network. ", fileHash)

	return nil
}

func (s *Server) HasFile(fileHash string) bool {
	return s.store.fileExists(fileHash)
}

// Stores a file and returns the file hash. Gossips the
// file content to other nodes for replicated storage.
func (s *Server) StoreFile(r io.Reader, propagate bool) (string, error) {

	// Store file to disk.
	fileHash, err := s.store.Write(r)
	if err != nil {
		return "", fmt.Errorf("failed to write file to disk: %w", err)
	}

	fileSize, err := s.store.GetFileSize(fileHash)
	if err != nil {
		return fileHash, fmt.Errorf("failed to get size of file: %w", err)
	}

	if !propagate {
		log.Printf("Successfully stored file (%s) to disk.", fileHash)
		return fileHash, nil
	}

	log.Printf("Successfully stored file (%s) to disk, replicating to peers...", fileHash)

	for _, peer := range s.peers.Values() {
		// We don't strictly need to put fileHash and fileSize in func args, but
		// since we are spawning goroutines in a loop, it is good practice.
		go func(peer p2p.Peer, fileHash string, fileSize int) {
			channel, err := peer.OpenChannel()
			if err != nil {
				log.Printf("failed to open channel with peer (%s): %s", peer.RemoteAddr(), err)
				return
			}

			defer channel.Close()

			hasFileRes, err := s.MakeHasFileRequest(fileHash, peer, channel.Id())
			if err != nil {
				log.Printf("error: hasFile request failed: %s", err)
				return
			}

			if hasFileRes.Result {
				// Peer already has file...
				return
			}

			putFileRes, err := s.MakePutFileRequest(fileHash, fileSize, peer, channel.Id(), propagate)
			if err != nil {
				log.Printf("error: failed making putFile request: %s", err)
				return
			}

			if !putFileRes.Result {
				// Peer rejected put request...
				return
			}

			// Peer accepted request, start stream
			f, err := s.store.Read(fileHash)
			if err != nil {
				log.Printf("error: failed reading file (%s): %s", fileHash, err)
				return
			}

			defer f.Close()

			_, err = io.Copy(channel, f)
			if err != nil {
				log.Printf("error: failed streaming file (%s): %s", fileHash, err)
				return
			}

			log.Printf("[%s]: Successfully streamed file (%s) to peer (%s)", s.Transport.LAddr(), fileHash, peer.RemoteAddr())

		}(peer, fileHash, int(fileSize))
	}

	return fileHash, nil
}

func (s *Server) GetFile(fileHash string) (io.ReadCloser, error) {
	if s.HasFile(fileHash) {
		return s.store.Read(fileHash)
	}

	log.Printf("(%s): File (%s) not found on disk, requesting file from peers...", s.Transport.LAddr(), fileHash)

	// Workflow for this:
	//	- Query all peers for file.
	//  - Mark file as incoming when we find a peer with the file.
	// 	- Issue getFile request to the peer.

	var success bool
	for _, peer := range s.peers.Values() {
		// go func(peer p2p.Peer, fileHash string) {
		// }(peer, fileHash)

		// Panics if the fileHash is different after fetching.
		if err := s.GetFileFromPeer(fileHash, peer); err != nil {
			log.Printf("failed to get file (%s) from peer (%s): %s", fileHash, peer.RemoteAddr(), err)
			continue
		}

		success = true
		break
	}

	if success {
		return s.store.Read(fileHash)
	}

	return nil, errors.New("failed to recieve file from network")
}

// Panics if file is different after writing to disk.
func (s *Server) GetFileFromPeer(fileHash string, peer p2p.Peer) error {

	channel, err := peer.OpenChannel()
	if err != nil {
		return fmt.Errorf("failed to open channel with peer (%s): %w", peer.RemoteAddr(), err)
	}

	defer channel.Close()

	hasFileRes, err := s.MakeHasFileRequest(fileHash, peer, channel.Id())
	if err != nil {
		return fmt.Errorf("failed making hasFile request: %w", err)

	}

	if !hasFileRes.Result {
		// Peer does not have file...
		return fmt.Errorf("peer (%s) does not have file: %s", peer.RemoteAddr(), fileHash)
	}

	// Request file from peer

	getFileRes, err := s.MakeGetFileRequest(fileHash, peer, channel.Id())
	if err != nil {
		return fmt.Errorf("failed making getFile request: %s", err)
	}

	// If we get here, request was successful, start reading from stream.
	fileSize := getFileRes.Result

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		_, err := io.CopyN(pw, channel, int64(fileSize))
		if err != nil {
			log.Printf("error: failed copying from channel: %s", err)
		}
	}()

	writtenHash, err := s.store.Write(pr)
	if err != nil {
		return fmt.Errorf("error: failed writing file: %s", err)
	}

	if writtenHash != fileHash {
		panic(fmt.Sprintf("wrong file hash after storage, expected (%s), got (%s)", fileHash, writtenHash))
	}

	log.Printf("Recieved file (%s) from peer (%s).", writtenHash, peer.RemoteAddr())

	return nil
}

// Deletes file locally.
func (s *Server) DeleteFile(fileHash string) error {
	if !s.HasFile(fileHash) {
		return nil
	}

	if err := s.store.Delete(fileHash); err != nil {
		return fmt.Errorf("error deleting file (%s): %w", fileHash, err)
	}

	return nil
}

// Deletes file locally and signals to peers that they
// should delete the file as well.
func (s *Server) PurgeFile(fileHash string) error {

	// Delete locally
	if err := s.DeleteFile(fileHash); err != nil {
		return fmt.Errorf("failed to delete file (%s): %w", fileHash, err)
	}

	// Send purge request to peers. I'm thinking that this should be
	// a blocking synchronous operation so that the caller can be
	// notified of success/failure after the message has propagated
	// through all peers.

	if err := s.NetworkPurgeFile(fileHash); err != nil {
		return fmt.Errorf("failed purging file (%s): %s", fileHash, err)
	}

	return nil
}

func (s *Server) OnPeer(peer p2p.Peer) error {
	s.addPeer(peer)
	peer.RegisterRequestHandler(s.handleRequest)
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

// Adds the value to the set if it is not already present.
//
// Returns a boolean indicating whether the value was added
// or not. Eg: Returns true when the value was not present and
// has been added. Returns false when the value is already
// present and has not been added.
func (s *set[T]) AddIfNotHas(t T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.m[t]; !ok {
		s.m[t] = struct{}{}
		return true
	}

	return false
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

// type syncMap[K comparable, V any] struct {
// 	m  map[K]V
// 	mu sync.RWMutex
// }

// func (s *syncMap[K, V]) Set(k K, v V) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	s.m[k] = v
// }

// func (s *syncMap[K, V]) Get(k K) (V, bool) {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()
// 	v, ok := s.m[k]
// 	return v, ok
// }

// func (s *syncMap[K, V]) Delete(k K) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	delete(s.m, k)
// }

// func (s *syncMap[K, V]) Has(k K) bool {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()
// 	_, ok := s.m[k]
// 	return ok
// }
