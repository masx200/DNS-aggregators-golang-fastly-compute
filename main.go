// https://developer.fastly.com/solutions/examples/DNS-aggregators-golang-fastly-compute/
package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	// "io"
	// "os"
	"github.com/miekg/dns"
	// "net/http"
	// "io"
	// "net/http"
	// "net/url"
	"encoding/base64"
	// "fmt"
	// "io"
	"encoding/json"
	"log"

	"github.com/fastly/compute-sdk-go/fsthttp"
	// "github.com/quic-go/quic-go/http3"
)

// BackendName is the name of our service backend.
// const BackendName = "origin_0"
type FastlyHttpMiddleWare = func(r *fsthttp.Request, next func() *fsthttp.Response) *fsthttp.Response

//	func DnsResolver(msg *dns.Msg) (res *dns.Msg, err error) {
//		return msg, nil
//	}
func CreateDOHMiddleWare(dnsResolver func(msg *dns.Msg) (*dns.Msg, error), getPathname func() string) FastlyHttpMiddleWare {
	var DohGetPost = func(r *fsthttp.Request, next func() *fsthttp.Response) *fsthttp.Response {
		var dnsParam = r.URL.Query().Get("dns")
		if len(dnsParam) > 0 && r.URL.Path == getPathname() && r.Method == "GET" {
			var buf []byte
			buf, err := base64.RawURLEncoding.DecodeString(dnsParam)
			if len(buf) == 0 || err != nil {
				log.Printf("dnsproxy: parsing dns request from get param %q: %v", dnsParam, err)
				// http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

				return &fsthttp.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusBadRequest) + "\n" + err.Error())),
				}
			}
			// http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			// http.Error(
			// 	w,
			// 	fmt.Sprintf("internal error: %s", err),
			// 	http.StatusInternalServerError,
			// )
			return handleDNSRequest(buf, dnsResolver)
		}
		return next()

	}
	return DohGetPost
}

func handleDNSRequest(buf []byte, dnsResolver func(msg *dns.Msg) (*dns.Msg, error)) *fsthttp.Response {
	var err error
	req := &dns.Msg{}
	if err = req.Unpack(buf); err != nil {
		log.Printf("dnsproxy: unpacking http msg: %s", err)

		return &fsthttp.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusBadRequest) + "\n" + err.Error())),
		}
	}
	res, err := dnsResolver(req)
	if err != nil {

		return &fsthttp.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
		}
	}
	buf, err = res.Pack()
	if err != nil {
		return &fsthttp.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
		}

	}
	return &fsthttp.Response{
		StatusCode: http.StatusOK,
		Header:     fsthttp.Header{"Content-Type": []string{"application/dns-message"}},
		Body:       io.NopCloser(bytes.NewReader(buf)),
	}
}

func main() {
	fsthttp.ServeFunc(func(ctx context.Context, w fsthttp.ResponseWriter, r *fsthttp.Request) {

		w.Header().Add("Alt-Svc", "h3=\":443\"; ma=86400")
		w.Header().Add("Alt-Svc", "h2=\":443\"; ma=86400")
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// This requires your service to be configured with a backend
		// named "origin_0" and pointing to "https://http-me.glitch.me".
		// if r.URL.Path == "/api/http" {

		// 	o := &fsthttp.BackendOptions{}
		// 	o.UseSSL(true).HostOverride("fastly-compute-hello-world-javascript.edgecompute.app").SNIHostname("fastly-compute-hello-world-javascript.edgecompute.app")
		// 	b, err := fsthttp.RegisterDynamicBackend(
		// 		"fastly-compute-hello-world-javascript.edgecompute.app",
		// 		"fastly-compute-hello-world-javascript.edgecompute.app",
		// 		o,
		// 	)
		// 	if err != nil {
		// 		log.Printf("Error happened in http request. Err: %s", err)
		// 		w.WriteHeader(fsthttp.StatusBadGateway)
		// 		w.Write([]byte("Bad Gateway" + "\n" + err.Error()))
		// 		return
		// 	}
		// 	req, err := fsthttp.NewRequest(r.Method, "https://fastly-compute-hello-world-javascript.edgecompute.app/", nil)

		// 	if err != nil {
		// 		log.Printf("Error happened in http request. Err: %s", err)
		// 		w.WriteHeader(fsthttp.StatusBadGateway)
		// 		w.Write([]byte("Bad Gateway" + "\n" + err.Error()))
		// 		return
		// 	}
		// 	req.Header = r.Header
		// 	resp, err := req.Send(ctx, b.Name())
		// 	if err != nil {
		// 		log.Printf("Error happened in http request. Err: %s", err)
		// 		w.WriteHeader(fsthttp.StatusBadGateway)
		// 		w.Write([]byte("Bad Gateway" + "\n" + err.Error()))
		// 		return
		// 	}
		// 	defer io.Copy(w, resp.Body)
		// 	w.WriteHeader(resp.StatusCode)

		// 	for k, v := range resp.Header {
		// 		for _, h := range v {
		// 			w.Header().Add(k, h)
		// 		}

		// 	}
		// 	return
		// }
		// If the URL path matches the path below, then return the client IP as a JSON response.
		// if r.URL.Path == "/api/clientIP" {
		// Get client IP address

		// if r.URL.Path == "/" {
		// 	var file, err = os.Open("./static/index.html")
		// 	if err != nil {
		// 		log.Println(err)
		// 		w.WriteHeader(fsthttp.StatusNotFound)
		// 		w.Write([]byte("Not Found\n" + err.Error()))
		// 		return
		// 	}
		// 	defer file.Close()
		// 	w.WriteHeader(fsthttp.StatusOK)
		// 	w.Header().Add("content-type", "text/html")
		// 	io.Copy(w, file)
		// 	return
		// }
		ClientIP := r.RemoteAddr
		resp := make(map[string]any)
		resp["request"] = map[string]any{
			"method":  r.Method,
			"url":     r.URL.String(),
			"headers": r.Header,
		}

		resp["client"] = map[string]any{"address": ClientIP}

		// resp]["Address"] = ClientIP
		JsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(JsonResp)
		w.Header().Set("Content-Type", "application/json")
		// Return 200 OK response
		w.WriteHeader(fsthttp.StatusOK)
		//return

		// }
		// resp, err := r.Send(ctx, BackendName)
		// if err != nil {
		// 	w.WriteHeader(fsthttp.StatusBadGateway)
		// 	fmt.Fprintln(w, err.Error())
		// 	return
		// }

		// w.Header().Reset(resp.Header)
		// w.WriteHeader(resp.StatusCode)
		// io.Copy(w, resp.Body)
	})
}
