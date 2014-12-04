package main

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func main() {
	log.Printf("Stage Multiplexer starting")
	http.HandleFunc("/teamcity/", handler)
	log.Fatal(http.ListenAndServe(":4000", nil))
}

type fakeResponse struct {
	Status int
}

func (r *fakeResponse) Header() http.Header {
	return make(http.Header)
}

func (r *fakeResponse) Write(data []byte) (int, error) {
	return len(data), nil
}

func (r *fakeResponse) WriteHeader(statusCode int) {
	r.Status = statusCode
}

func handler(rw http.ResponseWriter, r *http.Request) {
	file, err := os.Open(os.Getenv("SM_HOSTS_FILE"))
	if err != nil {
		log.Fatal(err)
		return
	}
	defer file.Close()

	log.Println("Processing request for " + r.URL.Path)

	var bodyData *bytes.Buffer
	bodyData, r.Body, err = drainBody(r.Body)
	if err != nil {
		log.Fatal("Error reading body: " + err.Error())
		return
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		host := strings.TrimSpace(scanner.Text())
		if host == "" {
			continue
		}
		u, err := url.Parse("http://" + host + "/")
		if err != nil {
			log.Println("Error: Couldn't parse host: " + host)
			continue
		}

		outreq := new(http.Request)
		*outreq = *r
		outreq.Host = u.Host
		outreq.Header.Set("Host", u.Host)
		if bodyData == nil {
			outreq.Body = nil
		} else {
			outreq.Body = ioutil.NopCloser(bytes.NewReader(bodyData.Bytes()))
		}

		go func() {
			log.Println("Sending request to", u.String())
			proxy := httputil.NewSingleHostReverseProxy(u)
			response := new(fakeResponse)
			proxy.ServeHTTP(response, outreq)
			log.Println("Finished request to host", host, "with status", response.Status)
		}()
	}

	rw.WriteHeader(http.StatusOK)
}

// taken from Go's httputil source
func drainBody(b io.ReadCloser) (data *bytes.Buffer, newCloser io.ReadCloser, err error) {
	if b == nil {
		return nil, nil, nil
	}
	var buff bytes.Buffer
	if _, err = buff.ReadFrom(b); err != nil {
		return nil, nil, err
	}
	if err = b.Close(); err != nil {
		return nil, nil, err
	}
	return &buff, ioutil.NopCloser(bytes.NewReader(buff.Bytes())), nil
}
