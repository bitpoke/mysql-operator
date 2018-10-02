package httpclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/google/go-querystring/query"
)

const (
	libraryVersion = "0.1.0"
	userAgent      = "appscode-httpclient/" + libraryVersion
	mediaType      = "application/json"
)

type Client struct {
	// HTTP client used to communicate with the microservice API.
	client *http.Client

	// Base URL for API requests.
	BaseURL string

	// User agent for client
	UserAgent string

	headers map[string]string

	b backoff.BackOff
}

func Default() *Client {
	return New(nil, nil, NewExponentialBackOff())
}

func New(client *http.Client, headers map[string]string, b backoff.BackOff) *Client {
	c := &Client{
		client:    client,
		UserAgent: userAgent,
		headers:   headers,
		b:         b,
	}
	if c.client == nil {
		c.client = &http.Client{Timeout: time.Second * 5}
	}
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	if c.b == nil {
		c.b = &backoff.StopBackOff{}
	}
	return c
}

func (c *Client) WithBaseURL(baseURL string) *Client {
	c.BaseURL = baseURL
	return c
}

func (c *Client) WithBasicAuth(username, password string) *Client {
	if username != "" && password != "" {
		c.headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	}
	return c
}

func (c *Client) WithBearerToken(token string) *Client {
	if token != "" {
		c.headers["Authorization"] = "Bearer " + token
	}
	return c
}

func (c *Client) WithTLSConfig(caCert []byte, clientPair ...[]byte) *Client {
	clientCert := []tls.Certificate{}
	switch len(clientPair) {
	case 2:
		if cert, err := tls.X509KeyPair(clientPair[0], clientPair[1]); err != nil {
			log.Fatal(err)
		} else {
			clientCert = append(clientCert, cert)
		}
	case 0:
		break
	default:
		log.Fatal(len(clientPair), "pem blocks provided for Client certificate pair instead of 2.")
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCert)
	c.client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: clientCert,
			RootCAs:      pool,
		},
	}
	return c
}

func (c *Client) WithInsecureSkipVerify() *Client {
	c.client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	return c
}

func (c *Client) WithUserAgent(ua string) *Client {
	c.UserAgent = ua
	return c
}

func (c *Client) WithTimeout(timeout time.Duration) *Client {
	c.client.Timeout = timeout
	return c
}

// NewRequest creates an API request. A relative URL can be provided in urlStr, which will be resolved to the
// BaseURL of the Client. Relative UfaRLS should always be specified without a preceding slash. If specified, the
// value pointed to by body is JSON encoded and included in as the request body.
func (c *Client) NewRequest(method, path string, request interface{}) (*http.Request, error) {
	var u *url.URL
	var err error

	if c.BaseURL != "" {
		u, err = url.Parse(c.BaseURL)
		if err != nil {
			return nil, err
		}
	}

	qv, err := query.Values(request)
	if err != nil {
		return nil, err
	}
	qs := qv.Encode()
	if qs != "" {
		if strings.Contains(path, "?") {
			path += "&" + qs
		} else {
			path += "?" + qs
		}
	}
	if path != "" {
		rel, err := url.Parse(path)
		if err != nil {
			return nil, err
		}
		if u != nil {
			u = u.ResolveReference(rel)
		} else {
			u = rel
		}
	}
	if u == nil {
		return nil, errors.New("No URL is provided.")
	}

	var body io.Reader
	if request != nil {
		if r, ok := request.(io.Reader); ok {
			body = r
		} else {
			var buf bytes.Buffer
			err := json.NewEncoder(&buf).Encode(request)
			if err != nil {
				return nil, err
			}
			body = &buf
		}
	}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add("User-Agent", c.UserAgent)
	for k, v := range c.headers {
		req.Header.Add(k, v)
	}
	return req, nil
}

// Do sends an API request and returns the API response. The API response is JSON decoded and stored in the value
// pointed to by v, or returned as an error if an API error has occurred. If v implements the io.Writer interface,
// the raw response will be written to v, without attempting to decode it.
func (c *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	var resp *http.Response
	var err error

	err = backoff.Retry(func() error {
		resp, err = c.client.Do(req)
		if err != nil {
			return err
		}
		if c := resp.StatusCode; c == 500 || c >= 502 && c <= 599 {
			// Avoid retry on 501: Not Implemented
			err = &status5xx{}
		}
		return err
	}, c.b)
	c.b.Reset()

	if err != nil {
		if _, ok := err.(*status5xx); !ok {
			return nil, err
		}
	}

	defer func() {
		if rerr := resp.Body.Close(); err == nil {
			err = rerr
		}
	}()

	err = checkResponse(resp)
	if err != nil {
		return resp, err
	}

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			_, err := io.Copy(w, resp.Body)
			if err != nil {
				return nil, err
			}
		} else {
			err := json.NewDecoder(resp.Body).Decode(v)
			if err != nil {
				return nil, err
			}
		}
	}
	return resp, err
}

type status5xx struct {
}

func (r *status5xx) Error() string {
	return "5xx Server Error"
}

// An ErrorResponse reports the error caused by an API request
type ErrorResponse struct {
	// HTTP response that caused this error
	Response *http.Response

	// Error message
	Message string `json:"message"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %v",
		r.Response.Request.Method, r.Response.Request.URL, r.Response.StatusCode, r.Message)
}

// CheckResponse checks the API response for errors, and returns them if present. A response is considered an
// error if it has a status code outside the 200 range. API error responses are expected to have either no response
// body, or a JSON response body that maps to ErrorResponse. Any other response body will be silently ignored.
func checkResponse(r *http.Response) error {
	if c := r.StatusCode; c >= 200 && c <= 299 {
		return nil
	}

	errorResponse := &ErrorResponse{Response: r}
	data, err := ioutil.ReadAll(r.Body)
	if err == nil && len(data) > 0 {
		errorResponse.Message = string(data)
	}
	return errorResponse
}

func (c *Client) Call(method, path string, reqBody, resType interface{}, needAuth bool) (*http.Response, error) {
	req, err := c.NewRequest(method, path, reqBody)
	if err != nil {
		return nil, err
	}
	if !needAuth {
		req.Header.Del("Authorization")
	}
	return c.Do(req, resType)
}
