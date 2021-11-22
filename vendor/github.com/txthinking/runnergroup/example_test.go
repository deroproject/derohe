package runnergroup_test

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/txthinking/runnergroup"
)

func Example() {
	g := runnergroup.New()

	s := &http.Server{
		Addr: ":9991",
	}
	g.Add(&runnergroup.Runner{
		Start: func() error {
			return s.ListenAndServe()
		},
		Stop: func() error {
			return s.Shutdown(context.Background())
		},
	})

	s1 := &http.Server{
		Addr: ":9992",
	}
	g.Add(&runnergroup.Runner{
		Start: func() error {
			return s1.ListenAndServe()
		},
		Stop: func() error {
			return s1.Shutdown(context.Background())
		},
	})

	go func() {
		time.Sleep(5 * time.Second)
		log.Println(g.Done())
	}()
	log.Println(g.Wait())
	// Output:
}
