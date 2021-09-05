package main

import (
	"fmt"
	pb "github.com/fran11388/grpc-chat/chat_service"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

type ChatServer struct {
	pb.UnimplementedChatServiceServer
}

type message struct {
	clientId string
	name     string
	text     string
}
type messageHandler struct {
	clientMsgMap map[string][]*message
	mu           sync.Mutex
}

var messageHandlerInstance = messageHandler{clientMsgMap:make(map[string][]*message)}

func (s *ChatServer) Chat(stream pb.ChatService_ChatServer) error {
	clientId := strconv.Itoa(rand.Intn(1e6))
	registerClient(clientId)
	errCh := make(chan error)

	go receiveFromStream(stream, clientId, errCh)

	go sendToStream(stream, clientId, errCh)

	return <-errCh
}

func registerClient(clientId string){
	log.Println(fmt.Sprintf("register client: %s",clientId))
	messageHandlerInstance.mu.Lock()
	messageHandlerInstance.clientMsgMap[clientId]=[]*message{}
	messageHandlerInstance.mu.Unlock()
}

func receiveFromStream(stream pb.ChatService_ChatServer, clientId string, errCh chan error) {
	for {
		in, err := stream.Recv()
		if err != nil {
			log.Println(fmt.Sprintf("err:%s",err))
			errCh <- err
			return
		}
		log.Println(fmt.Sprintf("receive, name:%s, text:%s\n",in.GetName(),in.GetText()))
		msg := &message{
			clientId: clientId,
			name:     in.GetName(),
			text:     in.GetText(),
		}
		pushMsgToOtherClient(clientId, msg)
	}
}

func sendToStream(stream pb.ChatService_ChatServer, clientId string, errCh chan error) {
	//loop determine have new msg
	for {
		time.Sleep(500 * time.Millisecond)
		messageHandlerInstance.mu.Lock()
		msgQue := messageHandlerInstance.clientMsgMap[clientId]
		if len(msgQue) > 0 {
			log.Println(fmt.Sprintf("client:%s, have new message",clientId))
			needSendMsgs := make([]*message, len(msgQue))
			//this prevents blocking other clients
			copy(needSendMsgs, msgQue)
			messageHandlerInstance.clientMsgMap[clientId] = []*message{}

			for _, msg := range needSendMsgs {
				res := &pb.ChatResponse{Name: msg.name, Text: msg.text}
				err := stream.Send(res)
				if err != nil {
					errCh <- err
					return
				}
				log.Println(fmt.Sprintf("sned msg to client:%s name:%s, text:%s",clientId,msg.name,msg.text))
			}
		}
		messageHandlerInstance.mu.Unlock()
	}
}

func pushMsgToOtherClient(senderClientId string, msg *message) {
	messageHandlerInstance.mu.Lock()
	clientMsgMap := messageHandlerInstance.clientMsgMap
	for clientId, queue := range clientMsgMap {
		if senderClientId != clientId {
			clientMsgMap[clientId] = append(queue, msg)
		}
	}
	messageHandlerInstance.mu.Unlock()
}

func main() {
	//todo enable ssl, lock performance improve, refactor code
	addr := "localhost:50051"
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Println("server listen : ", addr)
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterChatServiceServer(grpcServer, &ChatServer{})
	grpcServer.Serve(lis)
}
