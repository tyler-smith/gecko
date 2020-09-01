package pipe

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func broadcaster(socketName string, clientLen int) (chan (struct{}), error) {
	os.Remove(socketName)
	defer os.Remove(socketName)
	s := NewSocket()

	if err := s.Listen(socketName); err != nil {
		return nil, err
	}

	fmt.Println("Listening on", socketName)
	return make(chan (struct{})), nil
}

func TestPipe(t *testing.T) {
	// url, err := ioutil.TempFile("", "pipe")
	// if err != nil {
	// 	t.Fatal("Failed to create temp socket file:", err.Error())
	// }
	// url.Close()

	socketName := "/tmp/pip-test.sock"
	os.Remove(socketName)
	defer os.Remove(socketName)
	s := NewSocket()

	if err := s.Listen(socketName); err != nil {
		t.Fatal("Failed to listen on socket:", err.Error())
	}
	fmt.Println("Listening on", socketName)

	ready, err := broadcaster(socketName)
	if err != nil {
		t.Fatal("Failed to create broadcaster:", err.Error())
	}
	<-ready
	// go func() {
	// 	// s := NewSocket()
	// 	// err = s.Listen(url.Name())
	// 	// if err != nil {
	// 	// 	t.Fatal("Failed to listen on socket:", err.Error())
	// 	// }
	// 	// time.Sleep(1*time.Second)
	// 	if err := s.Send(context.Background(), []byte{0,1,2,3}); err != nil {
	// 		t.Fatal("Failed to send to socket:", err.Error())
	// 	}
	// }()
	//
	// time.Sleep(time.Millisecond)
	fmt.Println("Dialing", socketName)
	c, err := Dial(socketName)
	if err != nil {
		t.Fatal("Failed to dial socket:", err.Error())
	}

	time.Sleep(time.Millisecond)
	fmt.Println("Sending...")
	if err := s.Send(context.Background(), []byte{0, 1, 2, 3}); err != nil {
		t.Fatal("Failed to send to socket:", err.Error())
	}

	fmt.Println("Receiving...")
	msg, err := c.Recv(context.Background())
	if err != nil {
		t.Fatal("Failed to dial socket:", err.Error())
	}

	fmt.Println("Recejved")
	fmt.Println(msg)
}
