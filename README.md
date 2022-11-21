# Async-retry
Async-retry controls asynchronous retries in Go, and can be shutdown gracefully.

Main features of Async-retry are
* Disable cancellation of context passed to function as an argument
* Keep value of context passed to function as an argument
* Set timeout(default 10s) for each function call
* Recover from panic
* Control retry with delegating to https://github.com/avast/retry-go
* Gracefully shutdown

You can find other features or settings in options.go

# Example

```
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
			if err := asyncRetry.Do(
				ctx,
				func(ctx context.Context) error {
					// do task
					// ...
					return nil
				},
				func(err error) {
					if err != nil {
						log.Println(err.Error())
					}
				},
				asyncretry.Attempts(5),
				asyncretry.Timeout(8*time.Second),
			); err != nil {
				// asyncRetry is in shutdown
				log.Println(err.Error())
			}
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
```


# UseCase
We'll post a blog explains why we need this soon.

# Contributing
We are always welcome your contribution!

If you find bugs or want to add new features, please raise issues.

# License
MIT License

Copyright (c) 2022 Kyash