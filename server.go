package main

import (
	"context"
	"fmt"
	pb "github.com/fran11388/grpc-chat/chat_service"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
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
	ctx,cancel:=context.WithCancel(context.Background())
	defer cancel()
	go receiveFromStream(ctx,stream, clientId, errCh)
	go sendToStream(ctx,stream, clientId, errCh)

	return <-errCh
}

func registerClient(clientId string){
	log.Println(fmt.Sprintf("register client: %s",clientId))
	messageHandlerInstance.mu.Lock()
	defer messageHandlerInstance.mu.Unlock()
	messageHandlerInstance.clientMsgMap[clientId]=[]*message{}
}

func removeClient(clientId string){
	log.Println(fmt.Sprintf("remove client: %s",clientId))
	messageHandlerInstance.mu.Lock()
	defer messageHandlerInstance.mu.Unlock()
	delete(messageHandlerInstance.clientMsgMap,clientId)
}

func receiveFromStream(ctx context.Context,stream pb.ChatService_ChatServer, clientId string, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Println("close receiveFromStream(), clientId:",clientId)
			return
		default:
			in, err := stream.Recv()
			if err != nil {
				log.Println(fmt.Sprintf("receive stream err:%s",err))
				removeClient(clientId)
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
}

func sendToStream(ctx context.Context,stream pb.ChatService_ChatServer, clientId string, errCh chan error) {
	//loop determine have new msg
	for {
		select {
		case <-ctx.Done():
			log.Println("close sendToStream(), clientId:",clientId)
			return
		default:
			messageHandlerInstance.mu.Lock()
			msgQue := messageHandlerInstance.clientMsgMap[clientId]
			if len(msgQue) > 0 {
				log.Println(fmt.Sprintf("client:%s, have new message",clientId))
				messageHandlerInstance.clientMsgMap[clientId] = []*message{}
				for _, msg := range msgQue {
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
}

func pushMsgToOtherClient(senderClientId string, msg *message) {
	messageHandlerInstance.mu.Lock()
	defer messageHandlerInstance.mu.Unlock()
	clientMsgMap := messageHandlerInstance.clientMsgMap
	for clientId, queue := range clientMsgMap {
		if senderClientId != clientId {
			clientMsgMap[clientId] = append(queue, msg)
		}
	}
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
