package main

import (
// "errors"
)

// We need some method of keeping track of operations and message
// exchanges that happen between peers.
//
// One idea is to build a whole new set of messages that consist
// of a set of RPC methods and corresponding repsonses. We could
// do this useing gRPC and protos or just use structs and gob
// encoding.
//

type RPCMethod int

const (
	Unspecified RPCMethod = iota

	HasFile
	GetFile
	PutFile
	DeleteFile
	PurgeFile

	GetFileMetadata

	GetPeers

	GetDiskSpace
)

func (r RPCMethod) String() string {
	return []string{
		"RPCMethod_Unspecified",

		"RPCMethod_HasFile",
		"RPCMethod_GetFile",
		"RPCMethod_PutFile",
		"RPCMethod_DeleteFile",
		"RPCMethod_PurgeFile",

		"RPCMethod_GetFileMetadata",

		"RPCMethod_GetPeers",

		"RPCMethod_GetDiskSpace",
	}[r]
}

type RPCError struct {
	Err string
}

func (r RPCError) Error() string {
	return r.Err
}

func (r RPCError) IsNil() bool {
	return r.Err == ""
}

func (r RPCError) NotNil() bool {
	return r.Err != ""
}

var (
	// ErrRejected     = errors.New("the peer rejected the request")
	// ErrFileNotFound = errors.New("file not found")
	// ErrDiskSpace    = errors.New("not enough free disk space")
	// ErrHaveFile     = errors.New("file already stored")
	// ErrFailedDelete = errors.New("failed to delete file")
	// ErrAlreadyRecv  = errors.New("already receiving file")

	NilErr = RPCError{}

	ErrRejected     = RPCError{"the peer rejected the request"}
	ErrFileNotFound = RPCError{"file not found"}
	ErrDiskSpace    = RPCError{"not enough free disk space"}
	ErrHaveFile     = RPCError{"file already stored"}
	ErrFailedDelete = RPCError{"failed to delete file"}
	ErrAlreadyRecv  = RPCError{"already receiving file"}

	// ErrPurgeFailed    = errors.New("purge failed")
	ErrPurgeFailed = RPCError{"purge failed"}

	// ErrInternal = errors.New("internal server error")
	ErrInternal = RPCError{"internal server error"}
)

type RPCRequest struct {
	Sum isRPCRequest
}

type RPCResponse struct {
	Sum isRPCResponse
}

type isRPCRequest interface {
	isRPCRequest()
}

type isRPCResponse interface {
	isRPCResponse()
}

type HasFileRequest struct {
	FileHash string
}

type HasFileResponse struct {
	Result bool
	Err    RPCError
}

func (r *HasFileRequest) isRPCRequest()   {}
func (r *HasFileResponse) isRPCResponse() {}

type GetFileRequest struct {
	FileHash string
}

type GetFileResponse struct {
	Result int
	Err    RPCError
}

func (r *GetFileRequest) isRPCRequest()   {}
func (r *GetFileResponse) isRPCResponse() {}

type PutFileRequest struct {
	FileHash string
	FileSize int
}

type PutFileResponse struct {
	Result bool
	Err    RPCError
}

func (r *PutFileRequest) isRPCRequest()   {}
func (r *PutFileResponse) isRPCResponse() {}

type DeleteFileRequest struct {
	FileHash string
}

type DeleteFileResponse struct {
	Result bool
	Err    RPCError
}

func (r *DeleteFileRequest) isRPCRequest()   {}
func (r *DeleteFileResponse) isRPCResponse() {}

type PurgeFileRequest struct {
	FileHash string
}

type PurgeFileResponse struct {
	Result bool
	Err    RPCError
}

// type PurgeFileRequest struct {
// 	FileHash  string
// 	PeerAddrs []net.Addr
// }

// type PurgeFileResponse struct {
// 	Result PurgeFileResult
// 	Err    RPCError
// }

// type PurgeFileResult struct {
// 	Success   bool
// 	PeerAddrs []net.Addr
// 	FileHash  string
// }

func (r *PurgeFileRequest) isRPCRequest()   {}
func (r *PurgeFileResponse) isRPCResponse() {}

type GetDiskSpaceRequest struct{}

type GetDiskSpaceResponse struct {
	Result int
	Err    RPCError
}

func (r *GetDiskSpaceRequest) isRPCRequest()   {}
func (r *GetDiskSpaceResponse) isRPCResponse() {}
