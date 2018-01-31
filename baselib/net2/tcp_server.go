/*
 *  Copyright (c) 2018, https://github.com/nebulaim
 *  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package net2

import (
	"github.com/golang/glog"
	"net"
	"runtime/debug"
	"strings"
	"time"
	"io"
)

type TcpServer struct {
	connectionManager *ConnectionManager
	listener          net.Listener
	serverName        string
	// network           string
	// address           string
	protoName         string
	sendChanSize      int
	callback          TcpConnectionCallback
	running           bool
}

func NewTcpServer(listener net.Listener, serverName, protoName string, sendChanSize int, cb TcpConnectionCallback) *TcpServer {
	return &TcpServer{
		connectionManager: NewConnectionManager(),
		listener:          listener,
		serverName:        serverName,
		protoName:         protoName,
		sendChanSize:      sendChanSize,
		callback:          cb,
		running:           false,
	}
}

func (s *TcpServer) Serve() {
	if s.running {
		return
	}
	s.running = true

	for {
		conn, err := Accept(s.listener)
		if err != nil {
			glog.Error(err)
			return
		}

		// TODO(@benqi): limit maxConn
		codec, err := NewCodecByName(s.protoName, conn)
		if err != nil {
			conn.Close()
			return
		}

		tcpConn := NewServerTcpConnection(conn.(*net.TCPConn), s.sendChanSize, codec, s)
		go s.establishTcpConnection(tcpConn)
	}

	s.running = false
}

func Accept(listener net.Listener) (net.Conn, error) {
	var tempDelay time.Duration
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil, io.EOF
			}
			return nil, err
		}
		return conn, nil
	}
}

func (s *TcpServer) Stop() {
	if s.running {
		s.listener.Close()
		s.connectionManager.Dispose()
	}
}

func (s *TcpServer) Pause() {
}

func (s *TcpServer) OnConnectionClosed(conn Connection) {
	s.onConnectionClosed(conn.(*TcpConnection))
}

func (s *TcpServer) establishTcpConnection(conn *TcpConnection) {
	// glog.Info("establishTcpConnection...")
	defer func() {
		//
		if err := recover(); err != nil {
			glog.Errorf("tcp_server handle panic: %v\n%s", err, debug.Stack())
		}
	}()

	s.onNewConnection(conn)

	for {
		msg, err := conn.Receive()
		if err != nil {
			// glog.Errorf("recv error: %v", err)
			return
		}

		if msg == nil {
			glog.Errorf("recv a nil msg: %v", conn)
			// 是否需要关闭？
			return
		}

		if s.callback != nil {
			if err := s.callback.OnDataArrived(conn, msg); err != nil {
				// TODO: 是否需要关闭?
			}
		}
	}
}

func (s *TcpServer) onNewConnection (conn *TcpConnection) {
	if s.connectionManager != nil {
		s.connectionManager.putConnection(conn)
	}

	if s.callback != nil {
		s.callback.OnNewConnection(conn)
	}
}

func (s *TcpServer) onConnectionClosed (conn *TcpConnection) {
	if s.connectionManager != nil {
		s.connectionManager.delConnection(conn)
	}

	if s.callback != nil {
		s.callback.OnConnectionClosed(conn)
	}
}
