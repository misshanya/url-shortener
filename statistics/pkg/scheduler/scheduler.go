package scheduler

import "github.com/go-co-op/gocron/v2"

type Scheduler struct {
	cron gocron.Scheduler
	Tick chan struct{}
	job  gocron.Job
}

func NewScheduler(crontab string) (*Scheduler, error) {
	scheduler := &Scheduler{}

	cron, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	scheduler.cron = cron
	scheduler.Tick = make(chan struct{})

	j, err := cron.NewJob(
		gocron.CronJob(crontab, false),
		gocron.NewTask(
			func() {
				scheduler.Tick <- struct{}{}
			},
		),
	)
	if err != nil {
		return nil, err
	}
	scheduler.job = j

	return scheduler, nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Shutdown() error {
	return s.cron.Shutdown()
}
