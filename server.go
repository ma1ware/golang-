package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:8000")
	if err != nil {
		log.Fatal(err)
	}

	//发送消息到所有客户端
	go broadcaster()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		// 每个客户端一个goroutine
		go handleConn(conn)
	}
}

//channel 三种类型(只发送，只接收，既发送也接收)
//client 只发送不接收
type client chan<- string

var (
	entering = make(chan client)
	leaving  = make(chan client)
	message  = make(chan string)
)

func broadcaster() {
	clients := make(map[client]bool)
	for {
		select {
		case msg := <-message:
			for cli := range clients {
				cli <- msg
			}
		case cli := <-entering:
			clients[cli] = true
		case cli := <-leaving:
			delete(clients, cli)
			close(cli)
		}
	}
}

func handleConn(conn net.Conn) {
	ch := make(chan string)
	go writeToClient(conn, ch)
	who := conn.RemoteAddr().String()
	//当客户端连接上后，给客户端发送一条信息
	ch <- "You are " + who
	message <- who + " are arrived"
	entering <- ch
	input := bufio.NewScanner(conn)
	//阻塞监听客户端的输入
	for input.Scan() {
		message <- who + ": " + input.Text()
	}

	//客户端断开连接
	leaving <- ch
	message <- who + " are left"
	conn.Close()
}

func writeToClient(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		fmt.Fprintln(conn, msg)
	}
}
