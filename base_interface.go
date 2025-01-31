package sendgrid

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Version is this client library's current version
const (
	Version        = "3.9.0"
	rateLimitRetry = 5
	rateLimitSleep = 1100
)

type options struct {
	Auth     string
	Endpoint string
	Host     string
	Subuser  string
}

// Client is the Twilio SendGrid Go client
type Client struct {
	rest.Request
}

func (o *options) baseURL() string {
	return o.Host + o.Endpoint
}

// requestNew create Request
// @return [Request] a default request object
func requestNew(options options) rest.Request {
	requestHeaders := map[string]string{
		"Authorization": options.Auth,
		"User-Agent":    "sendgrid/" + Version + ";go",
		"Accept":        "application/json",
	}

	if len(options.Subuser) != 0 {
		requestHeaders["On-Behalf-Of"] = options.Subuser
	}

	return rest.Request{
		BaseURL: options.baseURL(),
		Headers: requestHeaders,
	}
}

// Send sends an email through Twilio SendGrid
func (cl *Client) Send(email *mail.SGMailV3) (*rest.Response, error) {
	cl.Body = mail.GetRequestBody(email)
	return MakeRequest(cl.Request)
}

// DefaultClient is used if no custom HTTP client is defined
var DefaultClient = rest.DefaultClient

// API sets up the request to the Twilio SendGrid API, this is main interface.
// Please use the MakeRequest or MakeRequestAsync functions instead.
// (deprecated)
func API(request rest.Request) (*rest.Response, error) {
	return MakeRequest(request)
}

// MakeRequest attempts a Twilio SendGrid request synchronously.
func MakeRequest(request rest.Request) (*rest.Response, error) {
	return DefaultClient.Send(request)
}

// MakeRequestRetry a synchronous request, but retry in the event of a rate
// limited response.
func MakeRequestRetry(request rest.Request) (*rest.Response, error) {
	retry := 0
	var response *rest.Response
	var err error

	for {
		response, err = MakeRequest(request)
		if err != nil {
			return nil, err
		}

		if response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}

		if retry > rateLimitRetry {
			return nil, errors.New("rate limit retry exceeded")
		}
		retry++

		resetTime := time.Now().Add(rateLimitSleep * time.Millisecond)

		reset, ok := response.Headers["X-RateLimit-Reset"]
		if ok && len(reset) > 0 {
			t, err := strconv.Atoi(reset[0])
			if err == nil {
				resetTime = time.Unix(int64(t), 0)
			}
		}
		time.Sleep(resetTime.Sub(time.Now()))
	}
}

// MakeRequestAsync attempts a request asynchronously in a new go
// routine. This function returns two channels: responses
// and errors. This function will retry in the case of a
// rate limit.
func MakeRequestAsync(request rest.Request) (chan *rest.Response, chan error) {
	r := make(chan *rest.Response)
	e := make(chan error)

	go func() {
		response, err := MakeRequestRetry(request)
		if err != nil {
			e <- err
		}
		if response != nil {
			r <- response
		}
	}()

	return r, e
}
