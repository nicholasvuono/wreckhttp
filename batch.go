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

type formattedResponse struct {
	Status        string
	Header        map[string][]string
	Body          string
	ContentLength int64
}

type response struct {
	Index    int
	Response http.Response
	Err      error
}

type batch struct {
	Requests []*http.Request
}

func Batch(requests []Request) (*batch, error) {
	batch := batch{}
	err := batch.SetRequests(requests)
	return &batch, err
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
		body, err := ioutil.ReadAll(res.Response.Body)
		explain(err)
		formattedResponse := formattedResponse{
			Status:        res.Response.Status,
			Header:        res.Response.Header,
			Body:          string(body),
			ContentLength: res.Response.ContentLength,
		}
		formattedResponses = append(formattedResponses, formattedResponse)
	}

	return formattedResponses
}

func (b *batch) GetRequests() []*http.Request {
	return b.Requests
}

func (b *batch) SetRequests(requests []Request) error {
	reqs := []*http.Request{}
	for _, req := range requests {
		body, err := json.Marshal(req.Body)
		if err != nil {
			return err
		}
		request, err := http.NewRequest(
			req.Method,
			req.URL,
			bytes.NewBuffer(body),
		)
		if err != nil {
			return err
		}
		request.Header = req.Headers
		reqs = append(reqs, request)
	}
	b.Requests = reqs
	return nil
}

func (b *batch) String() string {
	return fmt.Sprintf("%#v", b)
}
