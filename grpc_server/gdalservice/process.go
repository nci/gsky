package gdalservice

import (
	"context"
	//"log"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"bufio"
	"bytes"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
)

type ErrorMsg struct {
	Address string
	Replace bool
	Error   error
}

func createUniqueFilename(dir string) (string, error) {
	var filename string
	var err error

	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return filename, fmt.Errorf("Could not get the current working directory %v", err)
		}
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return filename, fmt.Errorf("The provided directory does not exist %v", err)
	}

	for {
		filename = filepath.Join(dir, strconv.FormatUint(rand.Uint64(), 16))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return filename, nil
		}
	}
}

type Task struct {
	Payload *GeoRPCGranule
	Resp    chan *Result
	Error   chan error
}

type Process struct {
	Context        context.Context
	CancelFunc     context.CancelFunc
	TaskQueue      chan *Task
	Address        string
	Cmd            *exec.Cmd
	CombinedOutput io.ReadCloser
	ErrorMsg       chan *ErrorMsg
}

func NewProcess(ctx context.Context, tQueue chan *Task, binary string, errChan chan *ErrorMsg, debug bool) *Process {

	newCtx, cancel := context.WithCancel(ctx)
	addr, err := createUniqueFilename("/tmp")
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(addr); !os.IsNotExist(err) {
		os.Remove(addr)
	}

	debugArg := ""
	if debug {
		debugArg = "-debug"
	}

	cmd := exec.CommandContext(newCtx, binary, "-sock", addr, debugArg)
	combinedOutput, err := cmd.StderrPipe()
	if err != nil {
		combinedOutput = nil
		log.Printf("Failed to obtain subprocess stderr pipe: %v\n", err)
	} else {
		cmd.Stdout = cmd.Stderr
	}

	return &Process{newCtx, cancel, tQueue, addr, cmd, combinedOutput, errChan}
}

func (p *Process) waitReady() error {
	timer := time.NewTimer(time.Millisecond * 200)
	ready := make(chan struct{})

	go func(signal chan struct{}) {
		for {
			time.Sleep(time.Millisecond * 20)
			if _, err := os.Stat(p.Address); err == nil {
				signal <- struct{}{}
				return

			}
		}
	}(ready)

	select {
	case <-ready:
		return nil
	case <-timer.C:
		return fmt.Errorf("Address file creation timed out")
	}
}

func (p *Process) Start() {
	err := p.Cmd.Start()
	if err != nil {
		p.ErrorMsg <- &ErrorMsg{p.Address, true, err}
		return
	}

	log.Println("Process running with PID", p.Cmd.Process.Pid)

	go func() {
		for task := range p.TaskQueue {

			conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: p.Address, Net: "unix"})
			if err != nil {
				task.Error <- fmt.Errorf("dial failed: %v", err)
				p.ErrorMsg <- &ErrorMsg{p.Address, true, err}
				break
			}

			inb, err := proto.Marshal(task.Payload)
			if err != nil {
				conn.Close()
				task.Error <- fmt.Errorf("encode failed: %v", err)
				continue
			}

			n, err := conn.Write(inb)
			if err != nil {
				conn.Close()
				task.Error <- fmt.Errorf("error writing %d bytes of data: %v", n, err)
				continue
			}
			conn.CloseWrite()

			var buf bytes.Buffer
			nr, err := io.Copy(&buf, conn)
			if err != nil {
				conn.Close()
				task.Error <- fmt.Errorf("error reading %d bytes of data: %v", nr, err)
				continue
			}
			conn.Close()

			out := new(Result)
			err = proto.Unmarshal(buf.Bytes(), out)
			if err != nil {
				task.Error <- fmt.Errorf("error decoding data: %v", err)
				continue
			}

			task.Resp <- out
		}
	}()

	go func() {
		// relay subprocess stderr and stdout to our stdout, with pid
		if p.CombinedOutput != nil {
			reader := bufio.NewReader(p.CombinedOutput)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}

				log.Println(p.Cmd.Process.Pid, line)
			}
		}

		err = p.Cmd.Wait()
		if err != nil {
			p.ErrorMsg <- &ErrorMsg{p.Address, false, fmt.Errorf("Failed to execute sub-process")}
			return
		}
		err = os.Remove(p.Address)
		if err != nil {
			p.ErrorMsg <- &ErrorMsg{p.Address, false, fmt.Errorf("Couldn't delete unix connection file")}
			return
		}

		select {
		case <-p.Context.Done():
			p.ErrorMsg <- &ErrorMsg{p.Address, false, p.Context.Err()}
		default:
			p.ErrorMsg <- &ErrorMsg{p.Address, true, fmt.Errorf("Process finished unexpectedly")}
		}
	}()
}

func (p *Process) Cancel() {
	p.CancelFunc()
	time.Sleep(100 * time.Millisecond)
}
