package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

func main() {
	threads := flag.Int("parallel", 10, "parallel requests count")
	flag.Parse()

	if *threads <= 0 {
		*threads = 10
	}

	out := makeRequests(flag.Args(), *threads)

	for res := range out {
		if res.err != nil {
			fmt.Println(res.err)
			continue
		}

		fmt.Printf("%s %x\n", res.address, res.hash)
	}
}

type result struct {
	address string
	hash    [16]byte
	err     error
}

func normalizeAddress(address string) string {
	if !(strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://")) {
		return "http://" + address
	}

	return address
}

func makeRequests(addresses []string, threads int) <-chan result {
	out := make(chan result)
	cli := &http.Client{Timeout: time.Minute}

	go func() {
		defer func() {
			close(out)
		}()

		sem := make(chan struct{}, threads)
		wg := &sync.WaitGroup{}

		wg.Add(len(addresses))

		for _, addr := range addresses {
			sem <- struct{}{}
			address := addr

			go func() {
				defer func() {
					<-sem
					wg.Done()
				}()

				address = normalizeAddress(address)
				res, err := cli.Get(address)

				if err != nil {
					out <- result{
						address: address,
						err:     fmt.Errorf("failed to make request: %w", err),
					}
					return
				}

				body, err := io.ReadAll(res.Body)
				if err != nil {
					out <- result{
						address: address,
						err:     fmt.Errorf("failed to read body: %w", err),
					}
					return
				}

				out <- result{
					address: address,
					hash:    md5.Sum(body),
				}
			}()
		}

		wg.Wait()
	}()

	return out
}
