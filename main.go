// https://developer.fastly.com/solutions/examples/DNS-aggregators-golang-fastly-compute/
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/fastly/compute-sdk-go/secretstore"

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
type DNSResult struct {
	Msg *dns.Msg
	Err error
}

func DnsResolver(msg *dns.Msg) (res *dns.Msg, err error) {
	res = msg
	var dohendpoions = GetDOH_ENDPOINT()
	if len(dohendpoions) == 0 {
		dohendpoions = []string{"https://doh.360.cn/dns-query"}
	}

	var results = []*dns.Msg{}
	var waitchan = make(chan DNSResult, len(dohendpoions))

	for _, doh := range dohendpoions {
		go func(doh string) {
			var res, err = DohClient(msg, doh)
			waitchan <- DNSResult{Msg: res, Err: err}
		}(doh)

	}

	for range dohendpoions {
		var result, ok = <-waitchan
		if ok {
			if result.Err != nil {
				fmt.Println(result.Err)
			} else {
				results = append(results, result.Msg)

			}
		}
	}
	if len(results) == 0 {
		return nil, errors.New("no dns result,all servers failure")
	}

	res = results[0]
	res.MsgHdr.Rcode = dns.RcodeSuccess
	for _, result := range results {
		log.Println(result)
	}
	answers := ArrayFlat(ArrayMap(results, func(msg *dns.Msg) []dns.RR {
		return msg.Answer
	}))
	var maxttl uint32 = ArrayReduce(answers, 0, func(a uint32, b dns.RR) uint32 {
		return uint32(math.Max(float64(a), float64(b.Header().Ttl)))
	})

	var messages = UniqBy(answers, func(rr dns.RR) string {
		rr.Header().Ttl = maxttl
		return rr.String()
	})
	res.Answer = messages
	return res, nil
}
func ArrayReduce[T any, U any](arr []T, initial U, fn func(U, T) U) U {
	result := initial

	for _, v := range arr {
		result = fn(result, v)
	}

	return result
}
func ArrayFlat[T any](arr [][]T) []T {
	result := make([]T, 0)

	for _, innerArr := range arr {
		result = append(result, innerArr...)
	}

	return result
}
func ArrayMap[T any, U any](arr []T, fn func(T) U) []U {
	result := make([]U, len(arr))

	for i, v := range arr {
		result[i] = fn(v)
	}

	return result
}
func UniqBy[T any](arr []T, fn func(T) string) []T {
	result := make([]T, 0)
	seen := make(map[string]bool)

	for _, v := range arr {
		key := fn(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}

	return result
}
func CreateDOHMiddleWare(dnsResolver func(msg *dns.Msg) (*dns.Msg, error), getPathname func() string) FastlyHttpMiddleWare {
	var DohGetPost = func(r *fsthttp.Request, next func() *fsthttp.Response) *fsthttp.Response {
		if !(r.URL.Path == getPathname()) {
			return next()
		}
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
		if contentType := r.Header.Get("Content-Type"); http.MethodPost == r.Method && contentType == "application/dns-message" {
			buf, err := io.ReadAll(r.Body)
			if len(buf) == 0 || err != nil {
				return &fsthttp.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusBadRequest) + "\n" + err.Error())),
				}
			}
			return handleDNSRequest(buf, dnsResolver)
		}
		return next()

	}
	return DohGetPost
}
func DohClient(msg *dns.Msg, dohServerURL string) (*dns.Msg, error) {
	/* 为了doh的缓存,需要设置id为0 ,可以缓存*/
	msg.Id = 0
	body, err := msg.Pack()
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, err
	}
	//http request doh
	req, err := fsthttp.NewRequest(http.MethodGet, dohServerURL, nil)
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, err
	}
	var query = req.URL.Query()
	query.Add("dns", base64.RawURLEncoding.EncodeToString(body))
	req.URL.RawQuery = query.Encode()
	res, err := req.Send(context.Background(), req.Host)
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, err
	}
	//res.status check
	if res.StatusCode != 200 {
		log.Println(dohServerURL, "http status code is not 200 "+fmt.Sprintf("status code is %d", res.StatusCode))
		return nil, fmt.Errorf("http status code is not 200" + fmt.Sprintf("status code is %d", res.StatusCode))
	}

	//check content-type
	if res.Header.Get("Content-Type") != "application/dns-message" {
		log.Println(dohServerURL, "content-type is not application/dns-message "+res.Header.Get("Content-Type"))
		return nil, fmt.Errorf(dohServerURL, "content-type is not application/dns-message "+res.Header.Get("Content-Type"))
	}
	//利用ioutil包读取百度服务器返回的数据
	data, err := io.ReadAll(res.Body)
	defer res.Body.Close() //一定要记得关闭连接
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, err
	}
	// log.Printf("%s", data)
	resp := &dns.Msg{}
	err = resp.Unpack(data)
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, err
	}
	return resp, nil
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
			log.Printf("Error happened in JSON marshal. Err: %s", err)
			// http.Error(
			// 	w,
			// 	fmt.Sprintf("internal error: %s", err),
			// 	http.StatusInternalServerError,
			// )
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
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
func GetDOH_ENDPOINT() []string {
	var store, err = secretstore.Open("DNS-aggregators-golang-fastly-compute")
	if err != nil {
		log.Println(err)
		return []string{}
	}
	s, err := store.Get("DOH_ENDPOINT")
	if err != nil {
		log.Println(err)
		return []string{}
	}
	v, err := s.Plaintext()
	if err != nil {
		log.Println(err)
		return []string{}
	}
	str := string(v)

	if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {

		//json
		var arr []string
		err = json.Unmarshal([]byte(str), &arr)
		if err != nil {
			log.Println(err)
			return []string{}
		}
		return arr
	} else {
		return []string{str}
	}

}
