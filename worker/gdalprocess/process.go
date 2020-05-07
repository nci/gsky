package gdalprocess

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"bufio"
	"bytes"
	"io"
	"log"

	"github.com/golang/protobuf/proto"
	pb "github.com/nci/gsky/worker/gdalservice"
)

type ErrorMsg struct {
	Address string
	Replace bool
	Error   error
}

type Task struct {
	Payload   *pb.GeoRPCGranule
	Resp      chan *pb.Result
	Error     chan error
	NumTrials int
}

type Process struct {
	TaskQueue        chan *Task
	Address          string
	TempFile         string
	Cmd              *exec.Cmd
	CombinedOutput   io.ReadCloser
	MaxTaskProcessed int
	ErrorMsg         chan *ErrorMsg
}

func NewProcess(tQueue chan *Task, binary string, port int, errChan chan *ErrorMsg, maxTaskProcessed int, debug bool) *Process {
	debugArg := ""
	if debug {
		debugArg = "-debug"
	}

	// we need to keep the temp file existing to prevent race condition
	// for creating unix socket for new processes
	tmpFile, err := ioutil.TempFile("", "gsky_rpc_")
	if err != nil {
		panic(err)
	}
	tmpFile.Close()
	tmpFileName := tmpFile.Name()
	addr := tmpFileName + "_socket"

	cmd := exec.Command(binary, "-sock", addr, debugArg)

	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	combinedOutput, err := cmd.StderrPipe()
	if err != nil {
		combinedOutput = nil
		log.Printf("Failed to obtain subprocess stderr pipe: %v\n", err)
	} else {
		cmd.Stdout = cmd.Stderr
	}

	return &Process{tQueue, addr, tmpFileName, cmd, combinedOutput, maxTaskProcessed, errChan}
}

func (p *Process) Start() error {
	err := p.Cmd.Start()
	if err != nil {
		p.RemoveTempFiles()
		p.ErrorMsg <- &ErrorMsg{p.Address, false, fmt.Errorf("Failed to start process: %v", err)}
		return err
	}

	log.Printf("Process running with PID:%d, Unix socket:%s, Max tasks:%d", p.Cmd.Process.Pid, p.Address, p.MaxTaskProcessed)

	go func() {
		defer p.RemoveTempFiles()

		nTrial := 0
		for {
			conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: p.Address, Net: "unix"})
			if err == nil {
				conn.Close()
				break
			}

			time.Sleep(10 * time.Millisecond)
			nTrial++

			if nTrial >= 5000 {
				syscall.Kill(p.Cmd.Process.Pid, syscall.SIGKILL)
				p.ErrorMsg <- &ErrorMsg{p.Address, false, fmt.Errorf("Failed to dial to process: %s", p.Address)}
				return
			}
		}

		taskProcessed := 0
		for task := range p.TaskQueue {
			conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: p.Address, Net: "unix"})
			if err != nil {
				p.retryTask(task, fmt.Errorf("dial failed: %v", err))
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
				p.retryTask(task, fmt.Errorf("conn.write failed: %v, bytes written: %v", err, n))
				break
			}
			conn.CloseWrite()

			var buf bytes.Buffer
			nr, err := io.Copy(&buf, conn)
			if err != nil {
				conn.Close()
				p.retryTask(task, fmt.Errorf("io.copy failed: %v, bytes read: %v", err, nr))
				break
			}
			conn.Close()

			out := new(pb.Result)
			err = proto.Unmarshal(buf.Bytes(), out)
			if err != nil {
				task.Error <- fmt.Errorf("error decoding data: %v", err)
				continue
			}

			if len(out.Error) == 0 {
				p.retryTask(task, fmt.Errorf("process communication error"))
				break
			}

			task.Resp <- out

			taskProcessed++
			if taskProcessed >= p.MaxTaskProcessed {
				syscall.Kill(p.Cmd.Process.Pid, syscall.SIGKILL)
				p.ErrorMsg <- &ErrorMsg{p.Address, false, fmt.Errorf("%d tasked processed, restarting process", taskProcessed)}
				break
			}
		}
	}()

	go func() {
		defer p.RemoveTempFiles()

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

func (p *Process) retryTask(task *Task, taskErr error) {
	syscall.Kill(p.Cmd.Process.Pid, syscall.SIGKILL)
	task.NumTrials++
	if task.NumTrials >= 5 {
		task.Error <- taskErr
	} else {
		p.TaskQueue <- task
	}
	p.ErrorMsg <- &ErrorMsg{p.Address, false, fmt.Errorf("Process IO failed: %v, retrying task: %v", taskErr, task.NumTrials)}
}

func (p *Process) RemoveTempFiles() {
	os.Remove(p.TempFile)
	os.Remove(p.Address)
}
