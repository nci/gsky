package gdalservice

import (
	"context"
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

func (p *ProcessPool) AddProcess(debug bool) {

	proc := NewProcess(context.Background(), p.TaskQueue, LibexecDir+"/gsky-gdal-process", p.ErrorMsg, debug)
	proc.Start()
	p.Pool = append(p.Pool, proc)
}

func CreateProcessPool(n int, debug bool) *ProcessPool {

	p := &ProcessPool{[]*Process{}, make(chan *Task, 400), make(chan *ErrorMsg)}

	go func() {
		for {
			select {
			case err := <-p.ErrorMsg:
				log.Println("Process needs to be restarted?", err)
			}
		}
	}()

	for i := 0; i < n; i++ {
		p.AddProcess(debug)
	}

	return p
}

func (p *ProcessPool) DeleteProcessPool() {
	for _, proc := range p.Pool {
		proc.Cancel()
	}
}
