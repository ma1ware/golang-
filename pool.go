package main

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

//连接池就是事先创建一个固定数量的连接资源池，需要时就取出来，不需要时就放回
//设置超时时间，若超时，则关闭连接

//只要类型实现了ConnRes接口中的方法，就认为是一个连接资源类型
type ConnRes interface {
	Close() error
}

//工厂方法，用于创建连接资源
type Factory func() (ConnRes, error)

//连接
type Conn struct {
	conn ConnRes
	time time.Time //连接时间
}

//连接池
type ConnPool struct {
	mu          sync.Mutex
	conns       chan *Conn    //通道
	factory     Factory       //创建连接资源
	closed      bool          //连接池是否关闭
	connTimeOut time.Duration //超时时间
}

//创建一个连接资源池
func NewConnPool(factory Factory, cap int, connTimeOut time.Duration) (*ConnPool, error) {
	if cap <= 0 {
		return nil, errors.New("cap不能小于0")
	}
	if connTimeOut <= 0 {
		return nil, errors.New("connTimeOut不能小于0")
	}

	cp := &ConnPool{
		mu:          sync.Mutex{},
		conns:       make(chan *Conn, cap),
		factory:     factory,
		closed:      false,
		connTimeOut: connTimeOut,
	}

	for i := 0; i < cap; i++ {
		// 通过工厂方法创建连接资源
		connRes, err := cp.factory()
		if err != nil {
			cp.Close()
			return nil, errors.New("factory出错")
		}
		//将连接资源插入通道中
		cp.conns <- &Conn{conn: connRes, time: time.Now()}
	}

	return cp, nil
}

//获取连接资源
func (cp *ConnPool) Get() (ConnRes, error) {
	if cp.closed {
		return nil, errors.New("连接池已关闭")
	}

	for {
		select {
		//从通道中获取连接资源
		case connRes, ok := <-cp.conns:
			{
				if !ok {
					return nil, errors.New("连接池已关闭")
				}
				//判断连接中的时间，如果超时则关闭
				//继续获取
				if time.Now().Sub(connRes.time) > cp.connTimeOut {
					connRes.conn.Close()
					continue
				}
				return connRes.conn, nil
			}
		default:
			{
				//若无法从通道中获取资源，则重新创建一个资源返回
				connRes, err := cp.factory()
				if err != nil {
					return nil, err
				}
				return connRes, nil
			}
		}
	}
}

//连接资源放回池中
func (cp *ConnPool) Put(conn ConnRes) error {
	if cp.closed {
		return errors.New("连接池已关闭")
	}
	select {
	//向通道中加入连接资源
	case cp.conns <- &Conn{conn: conn, time: time.Now()}:
		{
			return nil
		}
	default:
		{
			//若无法加入，则关闭连接
			conn.Close()
			return errors.New("连接池已满")
		}
	}
}

//关闭连接池
func (cp *ConnPool) Close() {
	if cp.closed {
		return
	}
	cp.mu.Lock()
	cp.closed = true
	//关闭通道
	close(cp.conns)
	for conn := range cp.conns {
		conn.conn.Close()
	}
	cp.mu.Unlock()
}

//返回池中通道长度
func (cp *ConnPool) len() int {
	return len(cp.conns)
}

//由于功能比较简单，代码量比较少，所以主函数不单独分开写
//main

func main() {
	cp, _ := NewConnPool(func() (ConnRes, error) {
		return net.Dial("tcp", ":8080")
	}, 10, 10*time.Second)

	//获取资源
	conn1, _ := cp.Get()
	conn2, _ := cp.Get()

	//连接池中资源大小为8
	fmt.Println("cp len : ", cp.len())
	conn1.(net.Conn).Write([]byte("hello"))
	conn2.(net.Conn).Write([]byte("world"))
	buf := make([]byte, 1024)
	n, _ := conn1.(net.Conn).Read(buf)
	fmt.Println("conn1 read: ", string(buf[:n]))
	n, _ = conn2.(net.Conn).Read(buf)
	fmt.Println("conn2 read: ", string(buf[:n]))

	//等待15s
	time.Sleep(time.Second * 15)
	//再从池中获取
	conn3, _ := cp.Get()
	//下面显示是0，因为池中的资源已经超时
	fmt.Println("cp len: ", cp.len())
	conn3.(net.Conn).Write([]byte("test"))
	n, _ = conn3.(net.Conn).Read(buf)
	fmt.Println("conn3 read: ", string(buf[:n]))

	//把三个连接资源放回池中
	cp.Put(conn1)
	cp.Put(conn2)
	cp.Put(conn3)
	//下面显示为3
	fmt.Println("cp len: ", cp.len())
	cp.Close()
}
