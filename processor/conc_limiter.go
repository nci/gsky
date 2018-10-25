package processor

import (
	"sync"
)

type ConcLimiter struct {
	*sync.WaitGroup
	Pool chan struct{}
}

func (c *ConcLimiter) Increase() {
	c.Add(1)
	c.Pool <- struct{}{}
}

func (c *ConcLimiter) Decrease() {
	select {
	case <-c.Pool:
		c.Done()
	default:
	}
}

func NewConcLimiter(cLevel int) *ConcLimiter {
	var wg sync.WaitGroup
	return &ConcLimiter{&wg, make(chan struct{}, cLevel)}
}
