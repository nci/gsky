package gdalservice

import (
	"context"
	"fmt"
	"log"
)

var LibexecDir = "/usr/local/libexec"

type ProcessPool struct {
	Pool      []*Process
	TaskQueue chan *Task
	ErrorMsg  chan *ErrorMsg
	//Health    chan *HealthMsg
}

func (p *ProcessPool) AddQueue(task *Task) {
	if len(p.TaskQueue) > 390 {
		task.Error <- fmt.Errorf("Pool TaskQueue is full")
		return
	}
	p.TaskQueue <- task
}

func (p *ProcessPool) AddProcess(debug bool) {

	proc := NewProcess(context.Background(), p.TaskQueue, LibexecDir + "/gsky-gdal-process", p.ErrorMsg, debug)
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

	/*
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT)
		go func() {
			for {
				select {
				case <-signals:
					p.DeleteProcessPool()
					time.Sleep(1 * time.Second)
					os.Exit(1)
				}
			}
		}()
	*/

	/*
		go func() {
			for {
				select {
				case hMsg := <-p.Health:
					p.RemoveProcess(hMsg.Address)
					if hMsg.Replace == true {
						p.AddProcess(p.Error, p.Health)
					}
				}
			}
		}()
	*/

	return p
}

/*
func (p *ProcessPool) RemoveProcess(address string) {
	newPool := []*Process{}
	for _, proc := range p.Pool {
		if proc.WarpAddress != address {
			newPool = append(newPool, proc)
		}
		if proc.DrillAddress != address {
			newPool = append(newPool, proc)
		}
	}
	p.Pool = newPool
}
*/

func (p *ProcessPool) DeleteProcessPool() {
	for _, proc := range p.Pool {
		proc.Cancel()
	}
}
