package asyncretry_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	asyncretry "github.com/Kyash/async-retry"
)

func ExampleAsyncRetry() {
	mux := http.NewServeMux()
	asyncRetry := asyncretry.NewAsyncRetry()
	mux.HandleFunc(
		"/hello",
		func(w http.ResponseWriter, r *http.Request) {
			var ctx = r.Context()
			go func() {
				err := asyncRetry.Do(
					ctx,
					func(ctx context.Context) error {
						// do task
						// ...
						return nil
					},
					asyncretry.Attempts(5),
					asyncretry.Timeout(8*time.Second),
				)
				if err != nil {
					log.Println(err.Error())
				}
			}()
			fmt.Fprintf(w, "Hello")
		},
	)
	srv := &http.Server{Handler: mux}

	ch := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}
		if err := asyncRetry.Shutdown(context.Background()); err != nil {
			log.Printf("AsyncRetry Shutdown: %v", err)
		}
		close(ch)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-ch
}
