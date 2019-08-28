// Copyright (c) 2019 Ashley Jeffs
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package input

import (
	"errors"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Jeffail/benthos/lib/log"
	"github.com/Jeffail/benthos/lib/message"
	"github.com/Jeffail/benthos/lib/metrics"
	"github.com/Jeffail/benthos/lib/response"
	"github.com/Jeffail/benthos/lib/types"
)

func TestTCPServerBasic(t *testing.T) {
	conf := NewConfig()
	conf.TCPServer.Address = "127.0.0.1:0"

	rdr, err := NewTCPServer(conf, nil, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}
	addr := rdr.(*TCPServer).Addr()

	defer func() {
		rdr.CloseAsync()
		if err := rdr.WaitForClose(time.Second); err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		if cerr, _ := conn.Write([]byte("foo\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("bar\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("baz\n")); err != nil {
			t.Error(cerr)
		}
		wg.Done()
	}()

	readNextMsg := func() (types.Message, error) {
		var tran types.Transaction
		select {
		case tran = <-rdr.TransactionChan():
			select {
			case tran.ResponseChan <- response.NewAck():
			case <-time.After(time.Second):
				return nil, errors.New("timed out")
			}
		case <-time.After(time.Second):
			return nil, errors.New("timed out")
		}
		return tran.Payload, nil
	}

	exp := [][]byte{[]byte("foo")}
	msg, err := readNextMsg()
	if err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	exp = [][]byte{[]byte("bar")}
	if msg, err = readNextMsg(); err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	exp = [][]byte{[]byte("baz")}
	if msg, err = readNextMsg(); err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	wg.Wait()
	conn.Close()
}

func TestTCPServerReconnect(t *testing.T) {
	conf := NewConfig()
	conf.TCPServer.Address = "127.0.0.1:0"

	rdr, err := NewTCPServer(conf, nil, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}
	addr := rdr.(*TCPServer).Addr()

	defer func() {
		rdr.CloseAsync()
		if err := rdr.WaitForClose(time.Second); err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		if cerr, _ := conn.Write([]byte("foo\n")); err != nil {
			t.Error(cerr)
		}
		conn.Close()
		conn, err = net.Dial("tcp", addr.String())
		if err != nil {
			t.Fatal(err)
		}
		if cerr, _ := conn.Write([]byte("bar\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("baz\n")); err != nil {
			t.Error(cerr)
		}
		wg.Done()
	}()

	readNextMsg := func() (types.Message, error) {
		var tran types.Transaction
		select {
		case tran = <-rdr.TransactionChan():
			select {
			case tran.ResponseChan <- response.NewAck():
			case <-time.After(time.Second):
				return nil, errors.New("timed out")
			}
		case <-time.After(time.Second):
			return nil, errors.New("timed out")
		}
		return tran.Payload, nil
	}

	exp := [][]byte{[]byte("foo")}
	msg, err := readNextMsg()
	if err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	exp = [][]byte{[]byte("bar")}
	if msg, err = readNextMsg(); err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	exp = [][]byte{[]byte("baz")}
	if msg, err = readNextMsg(); err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	wg.Wait()
	conn.Close()
}

func TestTCPServerMultipart(t *testing.T) {
	conf := NewConfig()
	conf.TCPServer.Address = "127.0.0.1:0"
	conf.TCPServer.Multipart = true

	rdr, err := NewTCPServer(conf, nil, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}
	addr := rdr.(*TCPServer).Addr()

	defer func() {
		rdr.CloseAsync()
		if err := rdr.WaitForClose(time.Second); err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		if cerr, _ := conn.Write([]byte("foo\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("bar\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("baz\n\n")); err != nil {
			t.Error(cerr)
		}
		wg.Done()
	}()

	readNextMsg := func() (types.Message, error) {
		var tran types.Transaction
		select {
		case tran = <-rdr.TransactionChan():
			select {
			case tran.ResponseChan <- response.NewAck():
			case <-time.After(time.Second):
				return nil, errors.New("timed out")
			}
		case <-time.After(time.Second):
			return nil, errors.New("timed out")
		}
		return tran.Payload, nil
	}

	exp := [][]byte{[]byte("foo"), []byte("bar")}
	msg, err := readNextMsg()
	if err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	exp = [][]byte{[]byte("baz")}
	if msg, err = readNextMsg(); err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	wg.Wait()
	conn.Close()
}

func TestTCPServerMultipartCustomDelim(t *testing.T) {
	conf := NewConfig()
	conf.TCPServer.Address = "127.0.0.1:0"
	conf.TCPServer.Multipart = true
	conf.TCPServer.Delim = "@"

	rdr, err := NewTCPServer(conf, nil, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}
	addr := rdr.(*TCPServer).Addr()

	defer func() {
		rdr.CloseAsync()
		if err := rdr.WaitForClose(time.Second); err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		if cerr, _ := conn.Write([]byte("foo@")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("bar@")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("@")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("baz\n@@")); err != nil {
			t.Error(cerr)
		}
		wg.Done()
	}()

	readNextMsg := func() (types.Message, error) {
		var tran types.Transaction
		select {
		case tran = <-rdr.TransactionChan():
			select {
			case tran.ResponseChan <- response.NewAck():
			case <-time.After(time.Second):
				return nil, errors.New("timed out")
			}
		case <-time.After(time.Second):
			return nil, errors.New("timed out")
		}
		return tran.Payload, nil
	}

	exp := [][]byte{[]byte("foo"), []byte("bar")}
	msg, err := readNextMsg()
	if err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	exp = [][]byte{[]byte("baz\n")}
	if msg, err = readNextMsg(); err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	wg.Wait()
	conn.Close()
}

func TestTCPServerMultipartShutdown(t *testing.T) {
	conf := NewConfig()
	conf.TCPServer.Address = "127.0.0.1:0"
	conf.TCPServer.Multipart = true

	rdr, err := NewTCPServer(conf, nil, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}
	addr := rdr.(*TCPServer).Addr()

	defer func() {
		rdr.CloseAsync()
		if err := rdr.WaitForClose(time.Second); err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		if cerr, _ := conn.Write([]byte("foo\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("bar\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("\n")); err != nil {
			t.Error(cerr)
		}
		if cerr, _ := conn.Write([]byte("baz\n")); err != nil {
			t.Error(cerr)
		}
		conn.Close()
		wg.Done()
	}()

	readNextMsg := func() (types.Message, error) {
		var tran types.Transaction
		select {
		case tran = <-rdr.TransactionChan():
			select {
			case tran.ResponseChan <- response.NewAck():
			case <-time.After(time.Second):
				return nil, errors.New("timed out")
			}
		case <-time.After(time.Second):
			return nil, errors.New("timed out")
		}
		return tran.Payload, nil
	}

	exp := [][]byte{[]byte("foo"), []byte("bar")}
	msg, err := readNextMsg()
	if err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	exp = [][]byte{[]byte("baz")}
	if msg, err = readNextMsg(); err != nil {
		t.Fatal(err)
	}
	if act := message.GetAllBytes(msg); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong message contents: %s != %s", act, exp)
	}

	wg.Wait()
}
