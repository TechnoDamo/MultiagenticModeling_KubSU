package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
	"sort"
)

type Job struct {
	id         int
	complexity int
}

type Agent struct {
	id            int
	jobs          chan Job
	isJobRun      bool
	jobStartTime  time.Time
	jobComplexity int
	totalJobs  int
	totalTime     int
	mu            sync.Mutex
}

func (a *Agent) Run(env *Environment, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range a.jobs {
		fmt.Printf("Agent %d starts job %d (complexity %d)\n", a.id, job.id, job.complexity)
		a.mu.Lock()
		a.jobStartTime = time.Now()
		a.jobComplexity = job.complexity
		a.isJobRun = true
		a.mu.Unlock()

		time.Sleep(time.Duration(job.complexity) * time.Second)

		a.mu.Lock()
		a.isJobRun = false
		a.totalJobs++
		a.totalTime += int(time.Since(a.jobStartTime).Seconds())
		a.mu.Unlock()

		fmt.Printf("Agent %d finished job %d\n", a.id, job.id)
		
		env.mu.Lock()
		env.jobsDone++
		if env.jobsDone >= env.jobsMax {
			fmt.Println("Max jobs processed, stopping system")
			env.cancel()
		}
		env.mu.Unlock()

	}
}

func (a *Agent) GetRemainingJobTime() (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isJobRun == false {
		return 0, fmt.Errorf("no job is running at the moment")
	}
	start := a.jobStartTime
	complexity := a.jobComplexity
	elapsed := int(time.Since(start).Seconds())
	remaining := complexity - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

type Environment struct {
	agents  []*Agent
	jobs    chan Job
	jobsMax int
	jobsDone int
	mu sync.Mutex
	cancel context.CancelFunc
}

func (env *Environment) createAgents(n int) {
	env.agents = make([]*Agent, n)

	for i := 0; i < n; i++ {
		agent := &Agent{
			id:   i + 1,
			jobs: make(chan Job, 10),
		}
		env.agents[i] = agent
	}
}

func (env *Environment) createJobs(ctx context.Context) {
	jobID := 1
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Job creation stopped due to context cancel")
			close(env.jobs) // optional: close jobs channel if no more jobs
			return
		default:
			// Random sleep between jobs (0.5s - 2s)
			sleepDuration := time.Duration(rand.Intn(1500)+500) * time.Millisecond
			time.Sleep(sleepDuration)

			// Random job complexity (1-10 seconds)
			complexity := rand.Intn(2) + 1
			job := Job{
				id:         jobID,
				complexity: complexity,
			}

			// Send job to the jobs channel (blocks if full)
			env.jobs <- job
			fmt.Printf("Created Job %d (complexity %d)\n", jobID, complexity)
			jobID++
		}
	}
}

func (env *Environment) distributeJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Distributor stopped: cancel signal received")
			for _, a := range env.agents {
				close(a.jobs)
			}
			return

		case job, ok := <-env.jobs:
			if !ok {
				fmt.Println("Distributor stopping: jobs channel closed")
				return
			}
			if env.jobsDone >= env.jobsMax {
				ctx.Done()
				fmt.Println("Max jobs processed, stopping system \n\n\n")
				return
			}
			agent, err := env.getAgentForJob()
			if err != nil {
				fmt.Println("No agent available")
				return
			} else {
				fmt.Printf("Agent %d selected\n", agent.id)
			}
			agent.jobs <- job
		}
	}
}

func (env *Environment) getAgentForJob() (*Agent, error) {
	if len(env.agents) == 0 {
		return nil, fmt.Errorf("no agents available")
	}

	minLoad := int(^uint(0) >> 1)
	var minAgent *Agent = env.agents[0]

	for _, a := range env.agents {
		load, err := a.GetRemainingJobTime()
		if err != nil {
			load = 0
		}

		if load < minLoad || (load == minLoad && a.id < minAgent.id) {
			minLoad = load
			minAgent = a
		}
	}

	if minAgent == nil {
		return nil, fmt.Errorf("no agent found")
	}
	return minAgent, nil
}

func (env *Environment) makeReport() {
	env.mu.Lock()
	defer env.mu.Unlock()

	sort.Slice(env.agents, func(i, j int) bool {
		ai := env.agents[i]
		aj := env.agents[j]

		ai.mu.Lock()
		aj.mu.Lock()
		defer ai.mu.Unlock()
		defer aj.mu.Unlock()

		if ai.totalJobs != aj.totalJobs {
			return ai.totalJobs > aj.totalJobs // decreasing
		}
		return ai.totalTime < aj.totalTime // increasing if totalJobs equal
	})

	fmt.Println("Agent stats:")
	for _, a := range env.agents {
		a.mu.Lock()
		fmt.Printf("Agent %d: totalJobs=%d, totalTime=%d\n", a.id, a.totalJobs, a.totalTime)
		a.mu.Unlock()
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())

	env :=  &Environment {
		jobs: make(chan Job, 20),
		jobsMax: 25, 
		cancel: cancel,
	}

	env.createAgents(10)

	var wg sync.WaitGroup
	for _, agent := range env.agents {
		wg.Add(1)
		go agent.Run(env, &wg)
	}

	go env.createJobs(ctx)
	go env.distributeJobs(ctx)

	wg.Wait()
	env.makeReport()
	fmt.Println()
}
