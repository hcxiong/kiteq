package main

import (
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"kiteq/client/chandler"
	"kiteq/client/core"
	"kiteq/client/listener"
	"kiteq/pipe"
	"kiteq/protocol"
	rclient "kiteq/remoting/client"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func buildStringMessage() *protocol.StringMessage {
	//创建消息
	entity := &protocol.StringMessage{}
	entity.Header = &protocol.Header{
		MessageId:   proto.String(messageId()),
		Topic:       proto.String("trade"),
		MessageType: proto.String("pay-succ"),
		ExpiredTime: proto.Int64(13700000000),
		GroupId:     proto.String("go-kite-test"),
		Commit:      proto.Bool(true)}
	entity.Body = proto.String("hello go-kite")

	return entity
}

var f, _ = os.OpenFile("/dev/urandom", os.O_RDONLY, 0)

func messageId() string {
	b := make([]byte, 16)
	f.Read(b)
	return fmt.Sprintf("%x", b)
}

func main() {

	c := flag.Int("c", 10, "-c=10")
	conn := flag.Int("conn", 1, "-conn=1")
	local := flag.String("local", "localhost:13800", "-local=localhost:13800")
	remote := flag.String("remote", "localhost:13800", "-remote=localhost:13800")
	flag.Parse()
	host, port, _ := net.SplitHostPort(*local)
	clients := make([]*core.KiteClient, 0, *conn)

	portv, _ := strconv.ParseInt(port, 10, 0)

	clientm := rclient.NewClientManager()
	cpipe := pipe.NewDefaultPipeline()
	cpipe.RegisteHandler("kiteclient-packet", chandler.NewPacketHandler("kiteclient-packet"))
	cpipe.RegisteHandler("kiteclient-accept", chandler.NewAcceptHandler("kiteclient-accept", &listener.ConsoleListener{}))
	cpipe.RegisteHandler("kiteclient-remoting", chandler.NewRemotingHandler("kiteclient-remoting", clientm))

	for i := 0; i < *conn; i++ {

		kclient := core.NewKitClient("user-service", "1234",
			net.JoinHostPort(host, strconv.Itoa(int(portv)+i)), *remote,
			func(remoteClient *rclient.RemotingClient, packet []byte) {
				event := pipe.NewPacketEvent(remoteClient, packet)
				err := cpipe.FireWork(event)
				if nil != err {
					log.Printf("KiteClient|onPacketRecieve|FAIL|%s|%t\n", err, packet)
				}
			})

		//开始向服务端发送数据

		clients = append(clients, kclient)
	}

	count := int32(0)
	lc := int32(0)

	fc := int32(0)
	flc := int32(0)

	go func() {
		for {

			tmp := count
			ftmp := fc

			time.Sleep(1 * time.Second)
			fmt.Printf("tps:%d/%d\n", (tmp - lc), (ftmp - flc))
			lc = tmp
			flc = ftmp
		}
	}()

	wg := &sync.WaitGroup{}

	stop := false
	for j := 0; j < *conn; j++ {
		for i := 0; i < *c; i++ {
			go func() {
				wg.Add(1)
				for !stop {
					idx := rand.Intn(len(clients))
					tmpclient := clients[idx]
					err := tmpclient.SendStringMessage(buildStringMessage())
					if nil != err {
						fmt.Printf("SEND MESSAGE |FAIL|%s\n", err)
						atomic.AddInt32(&fc, 1)
					} else {
						atomic.AddInt32(&count, 1)
					}
				}
				wg.Done()

			}()
		}
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Kill)

	select {
	//kill掉的server
	case <-ch:
		stop = true
	}

	wg.Wait()
	for _, v := range clients {
		v.Close()
	}

}