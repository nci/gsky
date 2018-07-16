package gdalservice

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"bufio"
	"bytes"
	"io"
	"log"

	"github.com/golang/protobuf/proto"
)

type ErrorMsg struct {
	Address string
	Replace bool
	Error   error
}

type Task struct {
	Payload *GeoRPCGranule
	Resp    chan *Result
	Error   chan error
}

type Process struct {
	TaskQueue      chan *Task
	TempFile       string
	Address        string
	Cmd            *exec.Cmd
	CombinedOutput io.ReadCloser
	ErrorMsg       chan *ErrorMsg
}

func NewProcess(tQueue chan *Task, binary string, errChan chan *ErrorMsg, debug bool) *Process {

	// we need to keep the temp file existing to prevent race condition
	// for creating unix socket for new processes
	tmpFile, err := ioutil.TempFile("", "gsky_rpc_")
	if err != nil {
		panic(err)
	}
	tmpFile.Close()
	tmpFileName := tmpFile.Name()
	addr := tmpFileName + "_socket"

	debugArg := ""
	if debug {
		debugArg = "-debug"
	}

	cmd := exec.Command(binary, "-sock", addr, debugArg)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	combinedOutput, err := cmd.StderrPipe()
	if err != nil {
		combinedOutput = nil
		log.Printf("Failed to obtain subprocess stderr pipe: %v\n", err)
	} else {
		cmd.Stdout = cmd.Stderr
	}

	return &Process{tQueue, addr, tmpFileName, cmd, combinedOutput, errChan}
}

func (p *Process) Start() error {
	err := p.Cmd.Start()
	if err != nil {
		os.Remove(p.TempFile)
		p.ErrorMsg <- &ErrorMsg{p.Address, false, fmt.Errorf("Failed to start process: %v", err)}
		return err
	}

	log.Println("Process running with PID", p.Cmd.Process.Pid)

	go func() {
		defer os.Remove(p.TempFile)
		defer os.Remove(p.Address)

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
		defer os.Remove(p.TempFile)
		defer os.Remove(p.Address)

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
			p.ErrorMsg <- &ErrorMsg{p.Address, true, fmt.Errorf("Process exited: %v", err)}
		}

	}()

	return nil
}
