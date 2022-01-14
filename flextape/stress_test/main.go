package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	fpb "github.com/enfabrica/enkit/flextape/proto"

	"google.golang.org/grpc"
)

var (
	numWorkers       = flag.Int("num_workers", 15, "Number of concurrent query runners")
	queriesPerWorker = flag.Int("queries_per_worker", 10, "Number of queries each runner should make")
	flextapeAddr     = flag.String("flextape_addr", "ft.corp.enfabrica.net", "Hostname/IP of Flextape server")
	flextapePort     = flag.Int("flextape_port", 8000, "Port number of Flextape server")
	queryTimeout     = flag.Int("query_timeout_seconds", 5, "Number of seconds before a query times out")
)

func runQuery() error {
	conn, err := grpc.Dial(net.JoinHostPort(*flextapeAddr, strconv.FormatInt(int64(*flextapePort), 10)), grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	client := fpb.NewFlextapeClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*queryTimeout)*time.Second)
	defer cancel()
	_, err = client.LicensesStatus(ctx, &fpb.LicensesStatusRequest{})
	if err != nil {
		return fmt.Errorf("failed to get licenses status: %w", err)
	}
	return nil
}

func worker(wg *sync.WaitGroup, num int, errChan chan error) {
	defer wg.Done()
	for i := 0; i < *queriesPerWorker; i++ {
		if err := runQuery(); err != nil {
			errChan <- fmt.Errorf("worker %d: %w", num, err)
		}
	}
}

func main() {
	flag.Parse()

	var wg sync.WaitGroup
	wg.Add(*numWorkers)
	errChan := make(chan error)
	doneChan := make(chan struct{})
	printChan := make(chan struct{})

	go func() {
		defer close(printChan)
		var numErrs int
		for {
			select {
			case err := <-errChan:
				fmt.Println(err)
				numErrs++
			case <-doneChan:
				fmt.Printf("Queries failed: %d\n", numErrs)
				return
			}
		}
	}()

	for i := 0; i < *numWorkers; i++ {
		go worker(&wg, i, errChan)
	}
	wg.Wait()
	close(doneChan)
	<-printChan
}
