package gdalservice

import (
	"fmt"
	"log"
)

var LibexecDir = "."

type ProcessPool struct {
	Pool      []*Process
	TaskQueue chan *Task
	ErrorMsg  chan *ErrorMsg
}

func (p *ProcessPool) AddQueue(task *Task) {
	if len(p.TaskQueue) > 390 {
		task.Error <- fmt.Errorf("Pool TaskQueue is full")
		return
	}
	p.TaskQueue <- task
}

func (p *ProcessPool) CreateProcess(executable string, debug bool) (*Process, error) {

	if len(executable) == 0 {
		executable = LibexecDir + "/gsky-gdal-process"
	}
	proc := NewProcess(p.TaskQueue, executable, p.ErrorMsg, debug)
	err := proc.Start()

	return proc, err
}

func CreateProcessPool(n int, executable string, debug bool) (*ProcessPool, error) {

	p := &ProcessPool{[]*Process{}, make(chan *Task, 400), make(chan *ErrorMsg)}

	go func() {
		for {
			select {
			case err := <-p.ErrorMsg:
				if err.Replace {
					log.Printf("Process: %v, %v, restarting...", err.Address, err.Error)
					for ip, proc := range p.Pool {
						if err.Address == proc.Address {
							p.Pool[ip] = nil
							proc, err := p.CreateProcess(executable, debug)
							if err == nil {
								p.Pool[ip] = proc
							}
							break
						}
					}
				} else {
					log.Printf("Process: %v, %v", err.Address, err.Error)
				}
			}
		}
	}()

	for i := 0; i < n; i++ {
		proc, err := p.CreateProcess(executable, debug)
		if err != nil {
			return nil, err
		}
		p.Pool = append(p.Pool, proc)
	}

	return p, nil
}
