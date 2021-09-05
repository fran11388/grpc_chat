package main

import (
	"fmt"
	pb "github.com/fran11388/grpc-chat/chat_service"
	"io"
	"math/rand"
	"strconv"
	"sync"
)

type ChatServer struct {
	pb.UnimplementedChatServiceServer
}

type message struct{
	clientId string
	name     string
	text string
}
type messageHandler struct {
	clientMsgMap map[string][]*message
	mu           sync.Mutex
}
var messageHandlerInstance=messageHandler{}

func (s *ChatServer) Chat(stream pb.ChatService_ChatServer) error {
	//為每一client生成unique id
	clientId := strconv.Itoa(rand.Intn(1e6))
	errCh := make(chan error)

	//持續確認用戶有沒有發訊息過來，有的話推送給其他用戶
	go receiveFromStream(stream, clientId, errCh)

	//持續確認有沒有需要發送給用戶的消息
	go sendToStream(stream, clientId, errCh)

	return <-errCh
}

func receiveFromStream(stream pb.ChatService_ChatServer, clientId string, errCh chan error) {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			errCh<-nil
		}
		if err != nil {
			errCh<-err
		}
		msg:=&message{
			clientId: clientId,
			name:in.GetName(),
			text: in.GetText(),
		}
		pushMsgToOtherClient(clientId,msg)
	}
}

func sendToStream(stream pb.ChatService_ChatServer, clientId string, errCh chan error) {
	//loop determine have new msg
	for {
		msgQue:=messageHandlerInstance.clientMsgMap[clientId]
		if len(msgQue)>0{
			messageHandlerInstance.mu.Lock()
			needSendMsgs:=make([]*message,len(msgQue))
			//this prevents blocking other clients
			copy(needSendMsgs,msgQue)
			messageHandlerInstance.clientMsgMap[clientId]=[]*message{}
			messageHandlerInstance.mu.Unlock()

			for _,msg:=range needSendMsgs{
				res:=&pb.ChatResponse{Name:msg.name,Text: msg.text}
				err := stream.Send(res)
				if err != nil {
					errCh<-err
					return
				}
			}
		}

	}
}


func pushMsgToOtherClient(senderClientId string,msg *message){
	messageHandlerInstance.mu.Lock()
	clientMsgMap:=messageHandlerInstance.clientMsgMap
	for clientId, queue:=range clientMsgMap{
		if senderClientId!=clientId{
			clientMsgMap[clientId]=append(queue,msg)
		}
	}
	messageHandlerInstance.mu.Lock()
}

func main() {
	fmt.Println("asd")
}
