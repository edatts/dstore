package main

import (
	"errors"
	"log"
	"net"
)

type PayloadType int

const (
	PayloadType_Unspecified PayloadType = iota

	PayloadType_Notify_NewFile
	PayloadType_Notify_HasFile
	PayloadType_Notify_NoFile
	PayloadType_Notify_DeleteSuccess
	PayloadType_Notify_DeleteFail

	// Maybe change Notify_NewFile to Request_SetFile
	// PayloadType_Request_SetFile
	PayloadType_Request_GetFile
	PayloadType_Request_DeleteFile

	PayloadType_Query_HasFile
)

func (t PayloadType) String() string {
	return []string{
		"PayloadType_Unspecified",

		"PayloadType_Notify_NewFile",
		"PayloadType_Notify_HasFile",
		"PayloadType_Notify_NoFile",
		"PayloadType_Notify_DeleteSuccess",
		"PayloadType_Notify_DeleteFail",

		"PayloadType_Request_GetFile",
		"PayloadType_Request_DeleteFile",

		"PayloadType_Query_HasFile",
	}[t]
}

type Payload struct {
	Type PayloadType
	Data PayloadData
}

type PayloadData struct {
	FileHash string
	Metadata Metadata
	StreamId uint32
}

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

var (
	ErrRejected       = errors.New("the peer rejected the request")
	ErrFileNotFound   = errors.New("file not found")
	ErrDiskSpace      = errors.New("not enough free disk space")
	ErrHaveFile       = errors.New("file already stored")
	ErrFailedDelete   = errors.New("failed to delete file")
	ErrAlreadyPurging = errors.New("already purging file")

	ErrInternal = errors.New("internal server error")
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
	Err    error
}

func (r *HasFileRequest) isRPCRequest()   {}
func (r *HasFileResponse) isRPCResponse() {}

type GetFileRequest struct {
	FileHash string
}

type GetFileResponse struct {
	Result int
	Err    error
}

func (r *GetFileRequest) isRPCRequest()   {}
func (r *GetFileResponse) isRPCResponse() {}

type PutFileRequest struct {
	FileHash string
	FileSize int
}

type PutFileResponse struct {
	Result bool
	Err    error
}

func (r *PutFileRequest) isRPCRequest()   {}
func (r *PutFileResponse) isRPCResponse() {}

type DeleteFileRequest struct {
	FileHash string
}

type DeleteFileResponse struct {
	Result bool
	Err    error
}

func (r *DeleteFileRequest) isRPCRequest()   {}
func (r *DeleteFileResponse) isRPCResponse() {}

type PurgeFileRequest struct {
	FileHash  string
	PeerAddrs []net.Addr
}

type PurgeFileResponse struct {
	Result PurgeFileResult
	Err    error
}

type PurgeFileResult struct {
	Success   bool
	PeerAddrs []net.Addr
	FileHash  string
}

func (r *PurgeFileRequest) isRPCRequest()   {}
func (r *PurgeFileResponse) isRPCResponse() {}

type GetDiskSpaceRequest struct{}

type GetDiskSpaceResponse struct {
	Result int
	Err    error
}

func (r *GetDiskSpaceRequest) isRPCRequest()   {}
func (r *GetDiskSpaceResponse) isRPCResponse() {}

// Another idea is to create a `NetworkOperation` struct that will
// keep track of operations that involve interacting with peers.
//
// For example, a query operation could be tracked simply by
// taking note of which peers were sent a query, which peers have
// responded to the query, and what their response was.
//
// A more complex operation such as a GetFile op would require
// multiple steps such as querying peers to see if they have the
// file, processing their responses, then requesting the file
// from a peer that has it.
//
//

type NetworkOperation struct {
	Id uint32
	Op OperationType
}

type OperationType byte

const (
	OperationType_Unspecified OperationType = iota
	OperationType_ReplicateFile
	OperationType_GetFile
	OperationType_PurgeFile
)

type NetOp_ReplicateFile struct{}
type NetOp_GetFile struct{}
type NetOp_PurgeFile struct{}

type Operation byte

const (
	Operation_Unspecified Operation = iota
	Operation_Notify
	Operation_Request
	Operation_Query
)

type NetOperation struct {
	Op   Operation
	Data OperationData
}

func (r *NetOperation) GetNotifyData() *NotificationData {
	if data, ok := r.Data.(*NotificationData); !ok {
		log.Printf("error: recieved an invalid RPC with operation type notify")
		return nil
	} else {
		return data
	}
}

func (r *NetOperation) GetRequestData() *RequestData {
	if data, ok := r.Data.(*RequestData); !ok {
		log.Printf("error: recieved an invalid RPC with operation type notify")
		return nil
	} else {
		return data
	}
}

func (r *NetOperation) GetQueryData() *QueryData {
	if data, ok := r.Data.(*QueryData); !ok {
		log.Printf("error: recieved an invalid RPC with operation type notify")
		return nil
	} else {
		return data
	}
}

type OperationData interface {
	isOperationData()
}

type NotificationData struct {
}

type RequestData struct {
}

type QueryData struct {
}

func (*NotificationData) isOperationData() {}
func (*RequestData) isOperationData()      {}
func (*QueryData) isOperationData()        {}

// type PayloadData_Notify_NewFile struct {
// 	FileHash string
// 	Metadata Metadata
// }

// type PayloadData_Request_GetFile struct {
// 	FileHash string
// }

// type PayloadData_Query_HasFile struct {
// 	FileHash string
// }

// func (PayloadData_Notify_NewFile) isPayloadData()  {}
// func (PayloadData_Request_GetFile) isPayloadData() {}
// func (PayloadData_Query_HasFile) isPayloadData()   {}
