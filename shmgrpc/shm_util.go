package shmgrpc

type QueueInfo struct {
	QueuePath     string
	QueueId       uint
	QueueReqType  uint
	QueueRespType uint
}

type ShmMessage struct {
	Method  string
	Payload interface{}
}
