package wreckhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
)

type Request struct {
	Method  string
	URL     string
	Headers map[string][]string
	Body    map[string]string
}

type batch struct {
	Requests []*http.Request
}

func Batch(requests []Request) *batch {
	batch := batch{}
	batch.SetRequests(requests)
	return &batch
}

func (b *batch) Send() []formattedResponse {

	client := http.DefaultClient

	semaphoreChan := make(chan struct{}, len(b.Requests))
	responsesChan := make(chan *response)

	defer func() {
		close(semaphoreChan)
		close(responsesChan)
	}()

	for i, req := range b.Requests {
		go func(i int, req *http.Request) {
			semaphoreChan <- struct{}{}
			res, err := client.Do(req)
			explain(err)
			response := &response{i, *res, err}
			responsesChan <- response
			<-semaphoreChan
		}(i, req)
	}

	var responses []response

	for {
		response := <-responsesChan
		responses = append(responses, *response)
		if len(responses) == len(b.Requests) {
			break
		}
	}

	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Index < responses[j].Index
	})

	return format(responses)
}

func format(responses []response) []formattedResponse {
	formattedResponses := []formattedResponse{}
	for _, res := range responses {
		body, err := ioutil.ReadAll(res.GetResponse().Body)
		explain(err)
		formattedResponse := formattedResponse{
			Status:        res.GetResponse().Status,
			Header:        res.GetResponse().Header,
			Body:          string(body),
			ContentLength: res.GetResponse().ContentLength,
		}
		formattedResponses = append(formattedResponses, formattedResponse)
	}

	return formattedResponses
}

func (b *batch) GetRequests() []*http.Request {
	return b.Requests
}

func (b *batch) SetRequests(requests []Request) {
	reqs := []*http.Request{}
	for _, req := range requests {
		body, err := json.Marshal(req.Body)
		explain(err)
		request, err := http.NewRequest(
			req.Method,
			req.URL,
			bytes.NewBuffer(body),
		)
		request.Header = req.Headers
		reqs = append(reqs, request)
	}
	b.Requests = reqs
}

func (b *batch) String() string {
	return fmt.Sprintf("%#v", b)
}
