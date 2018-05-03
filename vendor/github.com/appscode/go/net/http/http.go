package http

import (
	"net/http"
	"net/http/httputil"

	"github.com/golang/glog"
	"github.com/moul/http2curl"
)

// LogTransport logs http request and response at glog level 8. At level 10,
// response body will be also logged. At lower log level, this is zero cost.
// So, it is safe to always wrap http.DefaultTransport with LogTransport.
func LogTransport(t http.RoundTripper) http.RoundTripper {
	return &logTransport{Transport: t}
}

type logTransport struct {
	Transport http.RoundTripper
}

func (t *logTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if glog.V(8) {
		cmd, _ := http2curl.GetCurlCommand(request)
		glog.Infoln("request:", cmd)
	}
	resp, err := t.Transport.RoundTrip(request)
	if glog.V(8) && err == nil {
		if out, err := httputil.DumpResponse(resp, bool(glog.V(10))); err == nil {
			glog.V(8).Infoln("response:", string(out))
		}
	}
	return resp, err
}
