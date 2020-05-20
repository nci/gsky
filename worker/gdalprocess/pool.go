package gdalprocess

import (
	"fmt"
	"log"
	"math/rand"
)

const DefaultQueueSizePerProcess = 200

type ProcessPool struct {
	Pool             []*Process
	PoolSize         int
	TaskQueue        chan *Task
	MaxTaskProcessed int
	ErrorMsg         chan *ErrorMsg
}

func (p *ProcessPool) AddQueue(task *Task) {
	if len(p.TaskQueue) > DefaultQueueSizePerProcess*len(p.Pool)-10 {
		task.Error <- fmt.Errorf("Pool TaskQueue is full")
		return
	}
	p.TaskQueue <- task
}

func (p *ProcessPool) CreateProcess(executable string, port int, verbose bool) (*Process, error) {

	randTasks := rand.Intn(p.PoolSize)
	proc := NewProcess(p.TaskQueue, executable, port, p.ErrorMsg, p.MaxTaskProcessed+randTasks, verbose)
	err := proc.Start()

	return proc, err
}

func CreateProcessPool(n int, executable string, port int, maxTaskProcessed int, verbose bool) (*ProcessPool, error) {

	p := &ProcessPool{[]*Process{}, n, make(chan *Task, DefaultQueueSizePerProcess*n), maxTaskProcessed, make(chan *ErrorMsg)}

	go func() {
		for {
			select {
			case err := <-p.ErrorMsg:
				if err.Replace {
					if verbose {
						log.Printf("Process: %v, %v, restarting...", err.Address, err.Error)
					}
					for ip, proc := range p.Pool {
						if err.Address == proc.Address {
							p.Pool[ip] = nil
							proc, err := p.CreateProcess(executable, port, verbose)
							if err == nil {
								p.Pool[ip] = proc
							}
							break
						}
					}
				} else if verbose {
					log.Printf("Process: %v, %v", err.Address, err.Error)
				}
			}
		}
	}()

	for i := 0; i < n; i++ {
		proc, err := p.CreateProcess(executable, port, verbose)
		if err != nil {
			return nil, err
		}
		p.Pool = append(p.Pool, proc)
	}

	return p, nil
}
