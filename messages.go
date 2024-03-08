package main

import ()

type PayloadType int

const (
	PayloadType_Unspecified PayloadType = iota

	PayloadType_Notify_NewFile

	PayloadType_Request_GetFile

	PayloadType_Query_HasFile

	PayloadType_Response_HasFile
)

func (t PayloadType) String() string {
	return []string{
		"PayloadType_Unspecified",

		"PayloadType_Notify_NewFile",

		"PayloadType_Request_GetFile",

		"PayloadType_Query_HasFile",

		"PayloadType_Response_HasFile",
	}[t]
}

type Payload struct {
	Type PayloadType
	Data PayloadData
}

// func (p Payload) GetData() PayloadData {
// 	switch p.Type {
// 	case PayloadType_Notify_NewFile:
// 		return p.Data.(PayloadData_Notify_NewFile)

// 	case PayloadType_Request_GetFile:
// 		return p.Data.(PayloadData_Request_GetFile)

// 	case PayloadType_Query_HasFile:
// 		return p.Data.(PayloadData_Query_HasFile)

// 	}

// 	return nil
// }

type PayloadData interface {
	isPayloadData()
}

type PayloadData_Notify_NewFile struct {
	FileHash string
	Metadata Metadata
}

type PayloadData_Request_GetFile struct {
	FileHash string
}

type PayloadData_Query_HasFile struct {
	FileHash string
}

func (PayloadData_Notify_NewFile) isPayloadData()  {}
func (PayloadData_Request_GetFile) isPayloadData() {}
func (PayloadData_Query_HasFile) isPayloadData()   {}
