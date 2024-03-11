package main

import ()

type PayloadType int

const (
	PayloadType_Unspecified PayloadType = iota

	PayloadType_Notify_NewFile
	PayloadType_Notify_NoFile

	PayloadType_Request_GetFile

	PayloadType_Query_HasFile

	PayloadType_Response_HasFile
)

type PayloadDataa struct{}

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

// func (p Payload) GetData() (PayloadData, error) {
// 	switch p.Data.(type) {
// 	case PayloadData_Notify_NewFile:
// 		log.Println("NewFile notify type")

// 		return p.Data.(PayloadData_Notify_NewFile), nil

// 	case PayloadData_Request_GetFile:
// 		log.Println("GetFile request type")

// 		return p.Data.(PayloadData_Request_GetFile), nil

// 	case PayloadData_Query_HasFile:
// 		log.Println("HasFile query type")

// 		return p.Data.(PayloadData_Query_HasFile), nil

// 	case PayloadData_Response_HasFile:
// 		log.Println("HasFile response type")

// 		return p.Data.(PayloadData_Query_HasFile), nil
// 	}

// 	return nil, fmt.Errorf("unknown payload data type.")
// }

// type PayloadData interface {
// 	isPayloadData()
// }

type PayloadData struct {
	FileHash string
	Metadata Metadata
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

type PayloadData_Response_HasFile struct {
	FileHash string
}

func (PayloadData_Notify_NewFile) isPayloadData()   {}
func (PayloadData_Request_GetFile) isPayloadData()  {}
func (PayloadData_Query_HasFile) isPayloadData()    {}
func (PayloadData_Response_HasFile) isPayloadData() {}
