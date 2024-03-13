package main

import ()

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
}

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
