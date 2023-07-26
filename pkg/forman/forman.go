package forman

import (
	"time"

	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/utils"
	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/workers"

	"github.com/go-logr/logr"
)

type provider struct {
	working    bool
	next       *time.Time
	alarmClock *time.Timer
}

type Request struct {
	Name string `json:"name"`
}

type Result struct {
	Requeue bool
	After   time.Duration
}

func GoForman(
	log logr.Logger,
	workerCount int,
	reconcile func(req Request) Result,
) chan Request {
	requests := make(chan Request)

	go func() {
		toWorkers := make(chan Request)
		results := make(chan utils.Pair[Request, Result])
		providers := map[string]*provider{}

		sendToWorkers := func(req Request) {
			go func() { toWorkers <- req }()
		}

		log.Info("Starting", "workers", workerCount)
		for i := 0; i < workerCount; i++ {
			workers.GoWorker(toWorkers, results, reconcile)
		}

		log.Info("Waiting for incoming events")
		for {
			select {
			case req := <-requests:
				// Ensure a provider record exists in the providers maps
				p := providers[req.Name]
				if p == nil {
					p = &provider{}
					providers[req.Name] = p
				}

				// We are processing a new event so canceling the alarm
				if p.alarmClock != nil {
					p.alarmClock.Stop()
					p.alarmClock = nil
				}

				if p.working {
					p.next = utils.ToPointer(time.Now())
				} else {
					p.next = nil
					p.working = true
					sendToWorkers(req)
				}

			case pair := <-results:
				req, res := pair.Unpack()
				p := providers[req.Name]

				if p.next != nil && p.next.Before(time.Now()) {
					p.next = nil
					if p.alarmClock != nil {
						p.alarmClock.Stop()
						p.alarmClock = nil
					}
					sendToWorkers(req)
				} else {
					p.working = false
					if res.Requeue {
						when := time.Now().Add(res.After)
						if p.next == nil || p.next.After(when) {
							p.next = &when
							if p.alarmClock != nil {
								p.alarmClock.Stop()
							}
							p.alarmClock = time.AfterFunc(
								res.After,
								func() { requests <- req },
							)
						}
					}
				}
			}
		}
	}()

	return requests
}
