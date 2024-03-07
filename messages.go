package main

import ()

type PayloadType int

const (
	PayloadType_Unspecified PayloadType = iota
	PayloadType_Notify_NewFile
	PayloadType_RequestFile
)

func (t PayloadType) String() string {
	return []string{
		"PayloadType_Unspecified",
		"PayloadType_Notify_NewFile",
		"PayloadType_RequestFile",
	}[t]
}

type Payload struct {
	Type PayloadType
	Data PayloadData
}

func (p Payload) GetData() PayloadData {
	switch p.Type {
	case PayloadType_Notify_NewFile:
		return p.Data.(PayloadData_Notify_NewFile)
	case PayloadType_RequestFile:
		return p.Data.(PayloadData_RequestFile)
	}

	return nil
}

type PayloadData interface {
	isPayloadData()
}

type PayloadData_Notify_NewFile struct {
	FileHash string
	Metadata Metadata
}

type PayloadData_RequestFile struct {
	FileHash string
}

func (PayloadData_Notify_NewFile) isPayloadData() {}
func (PayloadData_RequestFile) isPayloadData()    {}
