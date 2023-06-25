package shmgrpc

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

type MessageHeader struct {
	Size int32
}

type Message struct {
	Header MessageHeader
	Data   [600]byte // Maximum payload size
}

type Queue struct {
	// ProducerFlag bool
	// ConsumerFlag bool
	mu          sync.Mutex
	Head        int32
	Tail        int32
	Count       int32
	TotalCount  int32
	BufferSize  int32
	StopPolling bool // DetachQueue chan bool
}

const (
	RequestKey  = 1234  // Shared memory key
	ResponseKey = 1235  // Shared memory key
	Mode        = 0644  // Permissions for shared memory
	Size        = 16384 //+ 40 // Shared memory size
	SegFlag     = IPC_CREAT | IPC_EXCL | Mode
	MessageSize = unsafe.Sizeof(Message{})
	QueueSize   = int32(Size) / int32(MessageSize)
)

// func main() {

// 	// shmid, shmaddr := InitializeShmRegion(Key, Size, uintptr(SegFlag))

// 	// defer remove(shmid)   // Remove shared memory segment when done
// 	// defer detach(shmaddr) //Detach shared memory segment when done

// 	// queuePtr := initializeQueue(shmaddr)

// 	// message := Message{
// 	// 	Header: MessageHeader{Size: 5}, Data: [1020]byte{'H', 'e', 'l', 'l', 'o'},
// 	// }

// 	// produceMessage(queuePtr, message)

// 	// read_message := consumeMessage(queuePtr)

// }

func produceMessage(queuePtr *Queue, message Message) {

	// for isFull(queuePtr) {
	// 	// Wait for space to become available
	// }

	// Wait until there's space in the circular buffer
poll:
	for {
		switch {
		case queuePtr.StopPolling:
			return
		default:
			// Wait for space to become available
			if isFull(queuePtr) {
				continue
			}
			break poll
		}

	}

	// Enqueue the message into the circular buffer
	enqueue(queuePtr, &message)
	// fmt.Printf("Producer: Message enqueued (Size: %s)\n", message.Data)

}

func consumeMessage(queuePtr *Queue) (Message, error) {
	var message Message

poll:
	for {
		switch {
		case queuePtr.StopPolling:
			//This might be problematic
			message = Message{}
			return message, errors.New("SharedMem detached")
		default:
			// Wait for space to become available
			if isEmpty(queuePtr) {
				continue
			} else {
				// Dequeue the message from the circular buffer
				message = dequeue(queuePtr)
				break poll
			}
		}
		// Wait for a message to become available
	}
	return message, nil
	// fmt.Printf("Consumer: Received message (Size: %d): %s\n", message.Header.Size, string(message.Data[:message.Header.Size]))
}

func initializeQueue(shmaddr uintptr) *Queue {
	// Initialize the circular buffer structure
	queue := Queue{
		Head:        0,
		Tail:        0,
		BufferSize:  QueueSize,
		Count:       0,
		TotalCount:  0,
		StopPolling: false,
	}
	fmt.Printf("Queue size: %d\n", unsafe.Sizeof(queue))
	queuePtr := GetQueue(shmaddr)
	*queuePtr = queue
	return queuePtr
}

func StopPollingQueue(queuePtr *Queue) {
	queuePtr.StopPolling = true
}

func InitializeShmRegion(key, size, segFlag uintptr) (uintptr, uintptr) {
	// Create a new shared memory segment
	shmid, _, errno := syscall.RawSyscall(syscall.SYS_SHMGET, key, size, segFlag)
	if errno != 0 {
		os.NewSyscallError("SYS_SHMGET", errno)
	}

	shmaddr, _, errno := syscall.RawSyscall(syscall.SYS_SHMAT, shmid, uintptr(0), segFlag)
	if errno != 0 {
		os.NewSyscallError("SYS_SHMAT", errno)
	}

	return shmid, shmaddr
}

func AttachToShmRegion(shmid, segFlag uintptr) uintptr {

	shmaddr, _, errno := syscall.RawSyscall(syscall.SYS_SHMAT, shmid, uintptr(0), segFlag)
	if errno != 0 {
		os.NewSyscallError("SYS_SHMAT", errno)
	}

	return shmaddr
}

func GetQueue(shmaddr uintptr) *Queue {
	queuePtr := (*Queue)(unsafe.Pointer(shmaddr))
	// fmt.Printf("unsafeGetBytes pointer: %p\n", &queuePtr)
	return queuePtr
}

func isFull(queue *Queue) bool {
	// queue.mu.Lock()
	isFull := (queue.Tail+1)%queue.BufferSize == queue.Head
	// queue.mu.Unlock()
	return isFull
}

func isEmpty(queue *Queue) bool {
	// queue.mu.Lock()
	isEmpty := queue.Head == queue.Tail
	// queue.mu.Unlock()
	return isEmpty
}

func enqueue(queue *Queue, message *Message) {
	// queue.mu.Lock()
	messageArray := (*[QueueSize]Message)(unsafe.Pointer(uintptr(unsafe.Pointer(queue)) + unsafe.Sizeof(*queue)))
	messageArray[queue.Tail] = *message
	queue.Count++
	queue.TotalCount++
	// fmt.Printf("Enqueue Count: %d\n", queue.Count)
	// fmt.Printf("Total Count: %d\n", queue.TotalCount)
	// fmt.Printf("Array pointer: %p\n", &messageArray[queue.Tail])
	// fmt.Printf("Array pos: %d\n", messageArray[queue.Tail])
	queue.Tail = (queue.Tail + 1) % queue.BufferSize
	// queue.mu.Unlock()
}

func dequeue(queue *Queue) Message {
	// queue.mu.Lock()
	message := (*[QueueSize]Message)(unsafe.Pointer(uintptr(unsafe.Pointer(queue)) + unsafe.Sizeof(*queue)))[queue.Head]
	queue.Head = (queue.Head + 1) % queue.BufferSize
	// queue.mu.Unlock()
	queue.Count--
	// fmt.Printf("Dequeue Count: %d\n", queue.Count)
	return message
}

// Detach used to detach from memory segment
func Remove(shm_id uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_SHMCTL, shm_id, 0, 0)
	if errno != 0 {
		return errors.New(errno.Error())
	}
	return nil
}

func Detach(shmaddr uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_SHMDT, shmaddr, 0, 0)
	if errno != 0 {
		return errors.New(errno.Error())
	}
	return nil
}
