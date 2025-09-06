package main

import (
	"fmt"
	"sync"
	"time"
)

type Client struct {
	ID         int
	Complexity int
}

type Agent struct {
	ID            int
	Queue         chan Client
	isJobRun      bool
	jobStartTime  time.Time
	jobComplexity int
	TotalClients  int
	TotalTime     int
	queryChan     chan chan int
	mu            sync.Mutex
}

func (a *Agent) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	for client := range a.Queue {
		fmt.Printf("Agent %d starts client %d (complexity %d)\n", a.ID, client.ID, client.Complexity)
		jobDone := make(chan struct{})
		a.mu.Lock()
		a.jobStartTime = time.Now()
		a.jobComplexity = client.Complexity
		a.mu.Unlock()
		go func(complexity int) {
			a.mu.Lock()
			a.isJobRun = true
			time.Sleep(time.Duration(complexity) * time.Second)
			a.isJobRun = false
			a.mu.Unlock()
			close(jobDone)
		}(client.Complexity)
		<-jobDone
		fmt.Printf("Agent %d finished client %d\n", a.ID, client.ID)
		a.TotalClients++
		a.TotalTime += int(time.Since(a.jobStartTime).Seconds())
	}
}

func (a *Agent) GetRemainingJobTime() (int, error) {
	a.mu.Lock()
	if a.isJobRun == false {
		return 0, fmt.Errorf("no job is running at the moment")
	}
	start := a.jobStartTime
	complexity := a.jobComplexity
	a.mu.Unlock()
	elapsed := int(time.Since(start).Seconds())
	remaining := complexity - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}
