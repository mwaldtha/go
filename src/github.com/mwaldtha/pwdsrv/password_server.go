package main

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	passwordFormName string = "password"
)

var hashes = make(map[int32]string)
var lock = sync.RWMutex{}
var jobCounter int32
var hashStats atomic.Value
var stopChan = make(chan os.Signal, 1)
var stopping int32

type HashStat struct {
	Total   int32   `json:"total"`
	Average float64 `json:"average"`
}
type HashHandler struct{}
type StatsHandler struct{}

func (hh HashHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//don't allow requests to process if the server is stopping
	if atomic.LoadInt32(&stopping) == 1 {
		http.Error(w, "Server is stopping.", http.StatusServiceUnavailable)
		return
	}

	log.Println("Begining hash request.")

	// only support POST and GET requests
	// return http.StatusMethodNotAllowed if any other HTTP method
	switch r.Method {
	case http.MethodGet:
		log.Println("Processing hash GET.")
		//find the job id in the URL
		urlParts := strings.Split(r.URL.Path, "/")
		if len(urlParts) >= 3 && urlParts[2] != "" {
			jid, err := strconv.Atoi(urlParts[2])
			if err != nil {
				http.Error(w, "Unable to process the supplied job id.", http.StatusBadRequest)
				return
			}

			log.Printf("Looking for job id %d.", jid)

			lock.RLock()
			jidHash := hashes[int32(jid)]
			lock.RUnlock()

			if jidHash == "" {
				http.Error(w, "Unable to find the specified job id.", http.StatusNotFound)
				return
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(jidHash))
			}
		} else {
			http.Error(w, "No job id specified.", http.StatusBadRequest)
			return
		}
	case http.MethodPost:
		log.Println("Processing hash POST.")
		hashStart := time.Now()
		p := r.PostFormValue(passwordFormName)
		if p != "" {
			//get the next job id
			jid := atomic.AddInt32(&jobCounter, 1)

			//call after the handler completes
			defer hashAndEncode(jid, p)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(strconv.Itoa(int(jid))))

			duration := time.Since(hashStart)
			log.Printf("Hash request duration (nanoseconds): %s", strconv.FormatInt(duration.Nanoseconds(), 10))
			updateStats(duration.Nanoseconds())
		} else {
			http.Error(w, "A value must be submitted.", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "Only POST and GET requests are supported.", http.StatusMethodNotAllowed)
	}

	log.Println("Finished hash request.")
}

func (sh StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//don't allow requests to process if the server is stopping
	if atomic.LoadInt32(&stopping) == 1 {
		http.Error(w, "Server is stopping.", http.StatusServiceUnavailable)
		return
	}

	log.Println("Begining stats request.")
	// only support GET requests
	// return http.StatusMethodNotAllowed if any other HTTP method
	switch r.Method {
	case http.MethodGet:
		log.Println("Processing stats GET.")
		//get the JSON to return to the client
		hsj, err := json.Marshal(hashStats.Load().(*HashStat))
		if err != nil {
			http.Error(w, "Unable to generate stats.", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(hsj))
	default:
		http.Error(w, "Only GET requests are supported.", http.StatusMethodNotAllowed)
	}

	log.Println("Finished stats request.")
}

func main() {
	// process command line flag for port number or default to 8080
	portFlag := flag.Int("port", 8080, "Port number the server will listen on.")

	flag.Parse()
	//initialize the WaitGroup used when stopping the server
	wg := sync.WaitGroup{}
	//initialise the hashStats object before the server starts
	hashStats.Store(&HashStat{Total: 0, Average: 0})

	hashListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *portFlag))
	if err != nil {
		log.Fatal(err)
	}

	//handle various interrupts to stop the server gracefully
	signal.Notify(stopChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stopChan
		//set global value used to indicate that the server is stopping
		atomic.StoreInt32(&stopping, 1)
		log.Println("Waiting for requests to finish.")
		wg.Wait()
		log.Println("Complete and shutting down.")
		os.Exit(1)
	}()

	//register with and without the trailing slash to avoid redirect in the case of a POST request
	http.Handle("/hash", HashHandler{})
	http.Handle("/hash/", HashHandler{})
	//only supporting GET requests, so the redirect when requested without the trailing slash is ok
	//but registering both to avoid the unnecessary redirect
	http.Handle("/stats", StatsHandler{})
	http.Handle("/stats/", StatsHandler{})

	server := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		//simple callback function for managing the WaitGroup used during shutdown
		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				wg.Add(1)
			case http.StateHijacked, http.StateClosed:
				wg.Done()
			}
		},
	}

	log.Printf("Starting server on port %d", *portFlag)
	err = server.Serve(hashListener)

	if err != nil {
		log.Fatal("An error was received during Serve: ", err)
	}
}

//private function to hash and encode the passed in data
//store the computed value in the global hashes map for the supplied key
func hashAndEncode(jid int32, data string) {
	// wait 5 seconds before starting
	time.Sleep(time.Duration(5) * time.Second)

	log.Printf("Hashing and encoding: '%s'", data)

	lock.Lock()

	h := sha512.New()
	h.Write([]byte(data))
	cs := h.Sum(nil)

	hashes[jid] = base64.StdEncoding.EncodeToString(cs)

	lock.Unlock()
}

//update the stats total count and average values based on the most resent duration
func updateStats(durationNano int64) {
	hs := hashStats.Load().(*HashStat)
	num := atomic.AddInt32(&hs.Total, 1)
	durationMilli := float64(durationNano) / 1e6
	avg := (((hs.Average * (float64(num) - 1)) + durationMilli) / float64(num))

	log.Printf("Adding duration (ms) %s to stats. New average: %s", strconv.FormatFloat(durationMilli, 'f', 5, 64), strconv.FormatFloat(avg, 'f', 5, 64))
	hashStats.Store(&HashStat{Total: num, Average: avg})
}
