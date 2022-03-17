package main

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"testing"
	"time"
)

func Test_makeRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "error parsing form", http.StatusBadRequest)
			return
		}

		delay := r.Form.Get("delay")

		dur, err := time.ParseDuration(delay)
		if err != nil {
			http.Error(w, "error parsing delay value", http.StatusBadRequest)
			return
		}

		time.Sleep(dur)

		r.Body = http.NoBody
	}))
	defer server.Close()

	tests := generateCases(server.URL)

	for i, test := range tests {
		idx := strconv.FormatInt(int64(i), 10)

		t.Run(idx, func(t *testing.T) {
			threads := random().Intn(3) + 1
			order, approxTime := simulate(threads, test)
			realOrder := make([]result, 0, len(test))

			start := time.Now()
			out := makeRequests(test, threads)

			for res := range out {
				realOrder = append(realOrder, res)
			}

			end := time.Now()

			if (end.Sub(start) - approxTime) > time.Second {
				t.Errorf("took too long")
			}

			for j := 0; j < len(order); j++ {
				if order[j] != realOrder[j] {
					t.Errorf("invalid order: \n%v\n%v\n", order[j], realOrder[j])
				}
			}
		})
	}
}

func random() *rand.Rand {
	src := rand.NewSource(time.Now().UnixNano())
	return rand.New(src)
}

// generateCases generates cases in the following format:
// [
// 		["localhost:8080?delay=1s"],
//		["localhost:8080?delay=2s"],
//		["localhost:8080?delay=200ms"],
// ]
func generateCases(serverAddress string) [][]string {
	// with the following cases the orders will match
	cases := [][]time.Duration{
		{
			time.Millisecond,
			time.Millisecond * 1000,
			time.Millisecond * 200,
			time.Millisecond * 800,
			time.Millisecond * 400,
			time.Millisecond * 1000,
			time.Millisecond * 1200,
		},
		{
			time.Millisecond * 900,
			time.Millisecond * 100,
			time.Millisecond * 500,
			time.Millisecond * 1100,
			time.Millisecond * 1300,
			time.Millisecond * 300,
		},
		{
			time.Millisecond * 750,
			time.Millisecond * 1150,
			time.Millisecond * 550,
			time.Millisecond * 250,
			time.Millisecond * 950,
			time.Millisecond * 1350,
		},
	}

	res := make([][]string, 0, len(cases))

	for _, delays := range cases {
		tmp := make([]string, 0, len(delays))

		for _, delay := range delays {
			tmp = append(tmp, fmt.Sprintf("%s?delay=%s", serverAddress, delay))
		}

		res = append(res, tmp)
	}

	return res
}

// simulate the behaviour of makeRequests and return approximated order of requests responses.
// The larger the difference between delays the more appropriate orders will come out
func simulate(threads int, addresses []string) (order []result, lasted time.Duration) {
	if threads < 1 {
		panic("threads num should be greater than 0")
	}
	if threads > len(addresses) {
		threads = len(addresses)
	}

	type pair struct {
		idx   int
		delay time.Duration
	}

	zeroHash := md5.Sum([]byte{})
	requests := make([]pair, 0, len(addresses))

	for idx, address := range addresses {
		u, err := url.Parse(address)
		if err != nil {
			panic(err)
		}

		delay, err := time.ParseDuration(u.Query().Get("delay"))
		if err != nil {
			panic(err)
		}

		requests = append(requests, pair{
			idx:   idx,
			delay: delay,
		})
	}

	batch := requests[:threads]

	sort.Slice(batch, func(i, j int) bool {
		return batch[i].delay < batch[j].delay
	})

	for i := threads; i < len(requests); i++ {
		min := batch[0].delay
		lasted += min

		for j := 0; j < threads; j++ {
			batch[j].delay -= min
		}

		order = append(order, result{
			address: addresses[batch[0].idx],
			hash:    zeroHash,
		})

		batch[0] = requests[i]

		sort.Slice(batch, func(i, j int) bool {
			return batch[i].delay < batch[j].delay
		})
	}

	for _, p := range batch {
		order = append(order, result{
			address: addresses[p.idx],
			hash:    zeroHash,
		})
	}

	lasted += batch[threads-1].delay

	return
}
