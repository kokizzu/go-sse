package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	sse "github.com/tmaxmax/go-sse/sse/server"

	"github.com/tmaxmax/go-sse/sse/server/field"
)

var eventHandler = sse.NewHandler(&sse.Configuration{
	Headers: map[string]string{
		"Access-Control-Allow-Origin": "*",
	},
	CloseEvent: sse.NewEvent(
		field.ID("CLOSE"),
		field.Text("We're done here\nGoodbye y'all!"),
	),
})

func main() {
	cancel := make(chan struct{})
	cancelMetrics := make(chan struct{})
	cancelSignal := make(chan os.Signal)
	signal.Notify(cancelSignal, os.Interrupt)

	go func() {
		select {
		case <-cancelMetrics:
			close(cancel)
		case <-cancelSignal:
			close(cancel)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/stop", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(cancelMetrics)

		w.WriteHeader(http.StatusOK)
	}))
	mux.Handle("/", SnapshotHTTPEndpoint)
	mux.Handle("/events", eventHandler)

	server := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: mux,
	}

	go eventHandler.StartWithSignal(cancel)

	go recordMetric("ops", time.Second*2, cancel)
	go recordMetric("cycles", time.Millisecond*500, cancel)

	go func() {
		for {
			r := 100 + rand.Int63n(1400)

			select {
			case <-time.After(time.Millisecond * time.Duration(r)):
				var fields []field.Field

				count := 1 + rand.Intn(5)

				for i := 0; i < count; i += 1 {
					fields = append(fields, field.Text(strconv.FormatUint(rand.Uint64(), 10)))
				}

				eventHandler.Send(sse.NewEvent(fields...))
			case <-cancel:
				return
			}
		}
	}()

	if err := runServer(server, cancel); err != nil {
		log.Println(err)
	}
}

func recordMetric(metric string, frequency time.Duration, cancel <-chan struct{}) {
	for {
		select {
		case <-time.After(frequency):
			v := Inc(metric)
			ev := sse.NewEvent(
				field.Name(metric),
				field.Text(strconv.FormatInt(v, 10)),
			)

			eventHandler.Send(ev)
		case <-cancel:
			break
		}
	}
}

func runServer(server *http.Server, cancel <-chan struct{}) error {
	shutdownError := make(chan error)

	go func() {
		<-cancel

		shutdownError <- server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return <-shutdownError
}
