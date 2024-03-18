package main

import (
	"errors"
	"fmt"
	"log"
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
	SendFile
	DeleteFile
	PurgeFile

	GetFileMetadata

	GetPeers

	GetDiskSpace
)

var (
	ErrFileNotFound = errors.New("file not found")
	ErrDiskSpace    = errors.New("not enough free disk space")
	ErrHaveFile     = errors.New("file already stored")
	ErrFailedDelete = errors.New("failed to delete file")

	ErrInternal = errors.New("internal server error")
)

type RPC struct {
	Method RPCMethod
	Sum    isRPCSum
	Err    error
}

type isRPCSum interface {
	isRPCSum()
}

type HasFileReq struct {
	FileHash string
}

type HasFileRes struct {
	FileHash string
	HasFile  bool
	FileSize int
}

func (r *HasFileReq) isRPCSum() {}
func (r *HasFileRes) isRPCSum() {}

type GetFileReq struct {
	FileHash string
}

type GetFileRes struct {
	FileHash string
	FileSize int
}

func (r *GetFileReq) isRPCSum() {}
func (r *GetFileRes) isRPCSum() {}

type SendFileReq struct {
	FileHash string
	FileSize int
}

type SendFileRes struct {
	FileHash string
}

func (r *SendFileReq) isRPCSum() {}
func (r *SendFileRes) isRPCSum() {}

type DeleteFileReq struct {
	FileHash string
}

type DeleteFileRes struct {
	FileHash string
	Success  bool
}

func (r *DeleteFileReq) isRPCSum() {}
func (r *DeleteFileRes) isRPCSum() {}

type PurgeFileReq struct {
	FileHash string
}

type PurgeFileRes struct {
	FileHash string
	Success  bool
}

func (r *PurgeFileReq) isRPCSum() {}
func (r *PurgeFileRes) isRPCSum() {}

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
