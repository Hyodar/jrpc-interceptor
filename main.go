package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
)

var (
	debug         bool
	listen        string
	listenHTTP    string
	usePrometheus bool
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      interface{} `json:"id"`
}

func main() {
	flag.StringVar(&listen, "listen", "127.0.0.1:9200", "ip:port to listen for mirrored messages")
	flag.StringVar(&listenHTTP, "listenHTTP", "0.0.0.0:9201", "ip:port to listen for http requests")
	flag.BoolVar(&usePrometheus, "usePrometheus", true, "Enable posting metrics to Prometheus")
	flag.BoolVar(&debug, "debug", false, "Enable debug")
	flag.Parse()

	if usePrometheus {
		go prometheusListener(listenHTTP)
	}

	http.HandleFunc("/", handleRequest)
	log.Fatal(http.ListenAndServe(listen, nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}

	var jrpcRequest JSONRPCRequest
	err = json.Unmarshal(body, &jrpcRequest)
	if err != nil {
		log.Printf("Error parsing JSON-RPC request: %v", err)
	}

	requestTime, _ := strconv.ParseFloat(r.Header.Get("X-Request-Time"), 64)
	responseTime, _ := strconv.ParseFloat(r.Header.Get("X-Response-Time"), 64)
	requestLength, _ := strconv.ParseInt(r.Header.Get("X-Request-Length"), 10, 64)
	bytesSent, _ := strconv.ParseInt(r.Header.Get("X-Bytes-Sent"), 10, 64)

	l := &logEntry{
		clientIP:          net.ParseIP(r.Header.Get("X-Real-IP")),
		method:            r.Method,
		uri:               r.URL.Path,
		scheme:            r.URL.Scheme,
		status:            r.Header.Get("X-Status"),
		duration:          requestTime,
		response_duration: responseTime,
		jrpc_method:       jrpcRequest.Method,
		bytesReceived:     uint64(requestLength),
		bytesSent:         uint64(bytesSent),
	}

	prometheusMetricsRegister(l)

	if debug {
		log.Printf("Processed request: %+v", l)
	}

	w.WriteHeader(http.StatusOK)
}
