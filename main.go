// https://developer.fastly.com/solutions/examples/DNS-aggregators-golang-fastly-compute/
package main

import (
	"bytes"
	"context"
	"crypto/sha512"
	// _ "embed"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fastly/compute-sdk-go/secretstore"

	// "io"
	// "os"
	"github.com/miekg/dns"
	// "net/http"
	// "io"
	// "net/http"
	// "net/url"
	"encoding/base64"
	"encoding/hex"
	// "fmt"
	// "io"
	"encoding/json"
	"log"

	"github.com/fastly/compute-sdk-go/cache/simple"
	"github.com/fastly/compute-sdk-go/fsthttp"
	// "github.com/quic-go/quic-go/http3"
)

/* //go:embed static/index.html
var indexhtmlByte []byte

//go:embed static/favicon.ico
var faviconByte []byte

//go:embed static/a8850f4365c46ec1.jpg_结果.webp
var a8850f4365c46ec1webp []byte

//go:embed static/EIse2e8XUAUWt8_结果.webp
var EIse2e8XUAUWt8webp []byte */

// ServeStatic 用于处理静态文件服务的请求。
// r: 表示客户端的请求。
// next: 是一个函数，当当前请求的路径找不到对应的静态文件时，会调用该函数继续处理请求。
// 返回值: 返回一个指向fsthttp.Response的指针，包含响应的状态码、头部和主体。
func ServeStatic(r *fsthttp.Request, next func(r *fsthttp.Request) *fsthttp.Response) *fsthttp.Response {
	// var filemap = map[string][]byte{
	// 	"/":                             indexhtmlByte,
	// 	"/favicon.ico":                  faviconByte,
	// 	"/a8850f4365c46ec1.jpg_结果.webp": a8850f4365c46ec1webp,
	// 	"/EIse2e8XUAUWt8_结果.webp":       EIse2e8XUAUWt8webp}
	// var typemap = map[string]string{
	// 	"/":                             "text/html",
	// 	"/favicon.ico":                  "image/x-icon",
	// 	"/a8850f4365c46ec1.jpg_结果.webp": "image/webp",
	// 	"/EIse2e8XUAUWt8_结果.webp":       "image/webp"}
	decodedString, err := url.QueryUnescape(r.URL.Path)
	if err != nil {
		fmt.Println("Error:", err)
		return &fsthttp.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
		}
	}
	log.Println("path:", r.URL.Path, "decoded:", decodedString)
	// var file, ok1 = filemap[decodedString]
	// var contenttype, ok2 = typemap[decodedString]
	if file != nil && ok2 && ok1 {
		// return &fsthttp.Response{
		// 	StatusCode: 200,
		// 	Header:     fsthttp.Header{"Content-Type": []string{contenttype}},
		// 	Body:       io.NopCloser(bytes.NewReader(file)),
		// }
	} else {
		return next(r)
	}

}

// BackendName is the name of our service backend.
// const BackendName = "origin_0"
// FastlyHttpMiddleWare 是一个类型，代表一个HTTP中间件函数。该函数接收一个fsthttp.Request指针和一个next函数，
// next函数当调用时返回一个fsthttp.Response指针。该中间件函数主要用来在请求处理流程中间插入自定义逻辑。
// 参数:
//
//	r *fsthttp.Request - 表示当前的HTTP请求。
//	next func() *fsthttp.Response - 一个函数，当调用时会继续处理HTTP请求，并返回一个HTTP响应。
//
// 返回值:
//
//	*fsthttp.Response - 表示处理后的HTTP响应。
type FastlyHttpMiddleWare = func(r *fsthttp.Request, next func(r *fsthttp.Request) *fsthttp.Response) *fsthttp.Response

// DOHRoundTripper 定义了一个自定义的 DNS Over HTTPs 请求和响应的传输器类型。
// 它是一个函数类型，接受一个 dns.Msg 结构体指针作为请求消息，
// 以及一个 map[string][]string 类型的 headers 作为 HTTP 请求头，
// 并返回一个 dns.Msg 结构体指针作为响应消息，一个 map[string][]string 类型的 headers 作为 HTTP 响应头，
// 以及一个 error 类型作为可能发生的错误。
type DOHRoundTripper = func(msg *dns.Msg, headers map[string][]string) (*dns.Msg, map[string][]string, error)

// DNSResult 结构体用于保存DNS查询的结果和可能发生的错误。
// 其中包含一个dns.Msg类型的Msg字段用于保存DNS消息体，以及一个error类型的Err字段用于保存查询过程中可能发生的错误。
type DNSResult struct {
	Msg    *dns.Msg // DNS查询的完整响应消息体
	Err    error    // DNS查询过程中发生的错误
	Header map[string][]string
}

// DnsResolver 是一个通过DOH（DNS over HTTPS）协议进行DNS解析的函数。
// 它接收一个dns.Msg类型的请求消息，并返回处理后的响应消息和可能发生的错误。
//
// 参数:
// msg *dns.Msg - 待处理的DNS请求消息。
//
// 返回值:
// *dns.Msg - 处理后的DNS响应消息。
// error - 解析过程中发生的错误（如果有）。
func DnsResolver(msg *dns.Msg, requestheaders map[string][]string) (*dns.Msg, map[string][]string, error) {
	var err error
	// var res = msg
	dohendpoions, err := GetDOH_ENDPOINT()
	if len(dohendpoions) == 0 || err != nil {
		log.Println("DOH_ENDPOINT is empty or error " + err.Error())
		dohendpoions = []string{"https://doh.360.cn/dns-query"}
	}
	log.Println("DOH_ENDPOINT:", dohendpoions)
	var results = []*dns.Msg{}
	var waitchan = make(chan *DNSResult, len(dohendpoions))

	for _, doh := range dohendpoions {
		go func(doh string) {
			var res, header, err = DohClientWithCache(msg, doh, requestheaders)
			waitchan <- &DNSResult{
				Msg:    res,
				Err:    err,
				Header: header,
			}
		}(doh)

	}
	var errs = []error{}
	var headers = []map[string][]string{}
	for range dohendpoions {
		var result, ok = <-waitchan
		if ok {
			if result.Err != nil {
				fmt.Println(result.Err)
				errs = append(errs, result.Err)
			} else {
				results = append(results, result.Msg)
				headers = append(headers, result.Header)
			}
		}
	}
	if len(results) == 0 {
		return nil, nil, errors.New("no dns result,all servers failure\n" + strings.Join(ArrayMap(errs, func(err error) string {

			return err.Error()
		}), "\n"))
	}
	//只处理A和aaaa记录

	if !(msg.Question[0].Qtype == dns.TypeA || msg.Question[0].Qtype == dns.TypeAAAA) {
		var index = 0

		for i := 0; i < len(results); i++ {
			if len(results[i].Answer) > len(results[index].Answer) {

				index = i

			}
		}
		return results[index], headers[index], nil
	}
	// res = results[0]
	/* 可能是dnssec的问题 */
	var res = msg.Copy()
	res.MsgHdr.Rcode = dns.RcodeSuccess
	res.MsgHdr.Response = true
	// res.Ns = results[0].Ns
	// res.Extra = results[0].Extra
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

		var oldttl = rr.Header().Ttl
		rr.Header().Ttl = maxttl
		var rrstr = rr.String()
		rr.Header().Ttl = oldttl
		return rrstr
	})
	res.Answer = messages
	var cnames = ArrayFilter(res.Answer, func(rr dns.RR) bool {
		return rr.Header().Rrtype == dns.TypeCNAME
	})
	var others = ArrayFilter(res.Answer, func(rr dns.RR) bool {
		return rr.Header().Rrtype != dns.TypeCNAME
	})
	//RandomShuffle(messages)
	/* 不能修改cname的顺序,否则会报错 */
	res.Answer = append(cnames, RandomShuffle(others)...)
	var responseheaders = http.Header{}
	for _, header := range headers {
		for k, values := range header {
			for _, v := range values {
				responseheaders.Add(k, v)
			}
		}

	}
	/* 删掉  Holds the RR(s) of the additional section.防止dnssec报错*/
	res.Extra = []dns.RR{}

	/* 需要 RecursionAvailable,否则客户端可能报错*/
	res.MsgHdr.RecursionAvailable = true
	res.Ns = results[0].Ns
	return res, responseheaders, nil
} // ArrayReduce 函数用于将数组中的元素逐步减少到一个单一的值，
// 通过应用一个提供的函数到数组的每个元素上，并将结果累积。
//
// 参数:
// T any - 数组元素的类型。
// U any - 初始值和累积结果的类型。
// arr []T - 待减少的数组。
// initial U - 函数计算的初始值。
// fn func(U, T) U - 一个函数，接受当前累积结果和数组元素作为参数，返回新的累积结果。
//
// 返回值:
// U - 经过所有数组元素处理后的累积结果。
func ArrayReduce[T any, U any](arr []T, initial U, fn func(U, T) U) U {
	result := initial

	for _, v := range arr {
		result = fn(result, v)
	}

	return result
} // ArrayFlat 是一个将二维切片展平为一维切片的函数。
// 参数 arr 是一个由 T 类型元素组成的二维切片。
// 返回值是一个由 T 类型元素组成的一维切片，包含了输入二维切片中的所有元素。
func ArrayFlat[T any](arr [][]T) []T {
	result := make([]T, 0)

	for _, innerArr := range arr {
		result = append(result, innerArr...)
	}

	return result
} // ArrayMap 函数接收一个数组和一个函数作为参数，将数组中的每个元素通过函数进行转换，并返回转换后的新数组。
// [T any] 和 [U any] 表示函数可以接受任何类型的数组和转换函数。
// arr 参数是待处理的数组。
// fn 参数是一个函数，用于对数组中的每个元素进行处理并返回一个新的值。
// 返回值是转换后的新数组。
func ArrayMap[T any, U any](arr []T, fn func(T) U) []U {
	result := make([]U, len(arr))

	for i, v := range arr {
		result[i] = fn(v)
	}

	return result
}

// UniqBy 函数通过指定的函数 fn 对切片 arr 中的元素进行去重处理。
//
// 参数:
// T any - arr 切片元素的类型，该类型可以是任意类型。
// Y comparable - fn 函数返回值的类型，该类型需要可比较。
// arr []T - 需要进行去重处理的切片。
// fn func(T) Y - 一个函数，用于从 arr 的每个元素中提取一个关键值（key），该关键值用于去重判断。
//
// 返回值:
// []T - 去重后的切片，保留了 arr 中第一次出现的每个元素。
func UniqBy[T any, Y comparable](arr []T, fn func(T) Y) []T {
	result := make([]T, 0)
	seen := make(map[Y]bool)

	for _, v := range arr {
		key := fn(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}

	return result
} // CreateDOHMiddleWare 创建一个用于处理DNS-over-HTTPs请求的中间件。
// dnsResolver: 一个函数，用于解析DNS请求并返回响应。它接收一个dns.Msg类型的参数，返回一个dns.Msg类型的响应和可能的错误。
// getPathname: 一个函数，用于获取DNS请求的URL路径。
// 返回值: 返回一个FastlyHttpMiddleWare类型的函数，该函数可用于处理HTTP请求。
func CreateDOHMiddleWare(dnsResolver DOHRoundTripper, getPathname func() string) FastlyHttpMiddleWare {
	var DohGetPost = func(r *fsthttp.Request, next func(r *fsthttp.Request) *fsthttp.Response) *fsthttp.Response {
		var requestheaders = r.Header.Clone()
		if !(r.URL.Path == getPathname()) {
			return next(r)
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
			return handleDNSRequest(buf, dnsResolver, requestheaders)
		}
		if contentType := r.Header.Get("Content-Type"); http.MethodPost == r.Method && contentType == "application/dns-message" {
			buf, err := io.ReadAll(r.Body)
			if len(buf) == 0 || err != nil {
				return &fsthttp.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusBadRequest) + "\n" + err.Error())),
				}
			}
			return handleDNSRequest(buf, dnsResolver, requestheaders)
		}
		return next(r)

	}
	return DohGetPost
}

// DohClient 是一个用于通过DOH（DNS over HTTPs）与DNS服务器进行通信的函数。
// 它发送一个DNS查询消息到指定的DOH服务器，并返回服务器的响应消息。
//
// 参数:
// msg: 指向dns.Msg的指针，包含要发送的DNS查询。
// dohServerURL: DOH服务器的URL，用于构建HTTP请求。
//
// 返回值:
// *dns.Msg: 指向接收到的DNS响应消息的指针。
// error: 如果在发送查询或处理响应时遇到错误，则返回错误信息；否则返回nil。
func DohClient(msg *dns.Msg, dohServerURL string, requestheaders map[string][]string) (*dns.Msg, map[string][]string, error) {
	log.Println("dohClient", dohServerURL)
	/* 为了doh的缓存,需要设置id为0 ,可以缓存*/
	msg.Id = 0
	body, err := msg.Pack()
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, nil, err
	}
	//http request doh
	req, err := fsthttp.NewRequest(http.MethodGet, dohServerURL, nil)
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, nil, err
	}
	var query = req.URL.Query()
	query.Add("dns", base64.RawURLEncoding.EncodeToString(body))
	query.Add("host", req.Host)
	req.URL.RawQuery = query.Encode()
	req.Header = requestheaders
	req.CacheOptions = fsthttp.CacheOptions{
		SurrogateKey: req.URL.String(),
	}
	res, err := req.Send(context.Background(), req.Host)
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, nil, err
	}
	//res.status check
	if res.StatusCode != 200 {
		log.Println(dohServerURL, "http status code is not 200 "+fmt.Sprintf("status code is %d", res.StatusCode))
		return nil, nil, fmt.Errorf("http status code is not 200" + fmt.Sprintf("status code is %d", res.StatusCode))
	}

	//check content-type
	if res.Header.Get("Content-Type") != "application/dns-message" {
		log.Println(dohServerURL, "content-type is not application/dns-message "+res.Header.Get("Content-Type"))
		return nil, nil, fmt.Errorf(dohServerURL, "content-type is not application/dns-message "+res.Header.Get("Content-Type"))
	}
	//利用ioutil包读取百度服务器返回的数据
	data, err := io.ReadAll(res.Body)
	defer res.Body.Close() //一定要记得关闭连接
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, nil, err
	}
	// log.Printf("%s", data)
	resp := &dns.Msg{}
	err = resp.Unpack(data)
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, nil, err
	}
	log.Println(resp.String())
	log.Println(res.Header)
	return resp, res.Header, nil
}

// handleDNSRequest 处理DNS请求
// buf []byte: 包含DNS请求数据的字节切片
// dnsResolver func(msg *dns.Msg) (*dns.Msg, error): 一个函数，用于解析DNS请求并返回响应
// 返回值 *fsthttp.Response: 返回一个HTTP响应，包含DNS响应数据或错误信息
func handleDNSRequest(reqbuf []byte, dnsResolver DOHRoundTripper, requestheaders map[string][]string) *fsthttp.Response {
	var err error
	req := &dns.Msg{}
	if err = req.Unpack(reqbuf); err != nil {
		log.Printf("dnsproxy: unpacking http msg: %s", err)

		return &fsthttp.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusBadRequest) + "\n" + err.Error())),
		}
	}
	res, upstreamresponseheaders, err := dnsResolver(req, requestheaders)
	if err != nil {

		return &fsthttp.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
		}
	}
	if !(req.Question[0].Qtype == dns.TypeA || req.Question[0].Qtype == dns.TypeAAAA) {
		resbuf, err := res.Pack()
		if err != nil {
			return &fsthttp.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
			}

		}
		return &fsthttp.Response{
			StatusCode: http.StatusOK,
			Header:     upstreamresponseheaders,
			Body:       io.NopCloser(bytes.NewReader(resbuf)),
		}
	}
	responseheaders := fsthttp.Header{}
	responseheaders.Set("Content-Type", "application/dns-message")
	var minttl = 600
	dohminttl, err := GetDOH_MINTTL()
	if err != nil {
		log.Println(err)
	} else {
		minttl = dohminttl
	}
	var minttl2 = minttl
	if len(res.Answer) > 0 {

		var minttl3 = int(math.Max(float64(minttl), float64(ArrayReduce(res.Answer, res.Answer[0].Header().Ttl, func(acc uint32, v dns.RR) uint32 {
			return uint32(math.Min(float64(acc), float64(v.Header().Ttl)))
		}))))
		responseheaders.Set("cache-control", "public,max-age="+fmt.Sprint(minttl3)+",s-maxage="+fmt.Sprint(minttl3))
	}
	for _, rr := range res.Answer {
		rr.Header().Ttl = uint32(math.Max(float64(minttl2), float64(rr.Header().Ttl)))
	}
	resbuf, err := res.Pack()
	if err != nil {
		return &fsthttp.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
		}

	}
	//提取cname 记录

	var cnames = ArrayFilter(res.Answer, func(rr dns.RR) bool {
		return rr.Header().Rrtype == dns.TypeCNAME
	})
	var others = ArrayFilter(res.Answer, func(rr dns.RR) bool {
		return rr.Header().Rrtype != dns.TypeCNAME
	})

	/* cname展平操作 */

	if len(cnames) > 1 && len(others) > 0 {
		var recordname = others[0].Header().Name
		for _, o := range others {
			o.Header().Name = recordname
		}
		cnames = []dns.RR{&dns.CNAME{Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeCNAME, Class: req.Question[0].Qclass, Ttl: uint32(minttl)}, Target: recordname}}
	} else if len(cnames) > 1 && len(others) == 0 {
		/* 为了解决浏览器对于cname记录重复的不兼容问题,需要把cname记录进行展平.还要把cname记录放在其他记录前面,不允许出现相同名字的cname记录. */
		cnames = UniqBy(cnames, func(rr dns.RR) string {
			return rr.Header().Name
		})
	}

	res.Answer = others
	/* 如果数据包太大,可能有兼容性问题 */
	/* 不能少了cname,否则会报错 */
	for len(resbuf) > 1024 {
		//删除res.Answer的后一半的一半
		res.Answer = res.Answer[:len(res.Answer)/2]
		resbuf, err = res.Pack()
		if err != nil {
			return &fsthttp.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
			}

		}
	}
	/* 测试是不是要把cname放在前面 */
	res.Answer = append(cnames, res.Answer...)
	for key, values := range upstreamresponseheaders {
		for _, v := range values {
			responseheaders.Add("x-debug-"+key, v)
		}
	}

	/* 修改dns结果后要重新生成buffer */
	resbuf, err = res.Pack()
	if err != nil {
		return &fsthttp.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(http.StatusText(http.StatusInternalServerError) + "\n" + err.Error())),
		}

	}
	responseheaders.Set("content-length", fmt.Sprint(len(resbuf)))
	log.Println("handleDNSRequest", res.String())
	log.Println("handleDNSRequest", responseheaders)
	return &fsthttp.Response{
		StatusCode: http.StatusOK,
		Header:     responseheaders,
		Body:       io.NopCloser(bytes.NewReader(resbuf)),
	}
}

func main() {
	fsthttp.ServeFunc(func(ctx context.Context, w fsthttp.ResponseWriter, r *fsthttp.Request) {
		log.Println(r.URL, r.Method)
		log.Println(r.Header)
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

		// var notfound = false

		var response = Forwarded()(r, func(r *fsthttp.Request) *fsthttp.Response {
			return CreateDOHMiddleWare(DnsResolver, func() string {

				var PATHNAME, err = GetDOH_PATHNAME()
				if err != nil {
					log.Println(err)
					return "/"
				} else {
					return PATHNAME
				}
				// return "/"
			})(r, func(r *fsthttp.Request) *fsthttp.Response {

				return ServeStatic(r, func(r *fsthttp.Request) *fsthttp.Response {
					return &fsthttp.Response{StatusCode: fsthttp.StatusNotFound}
				})
				// notfound = true

			})
		})
		if /* notfound && */ response.StatusCode == fsthttp.StatusNotFound {
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

			w.Header().Set("Content-Type", "application/json")
			// Return 200 OK response
			w.WriteHeader(fsthttp.StatusOK)
			//return
			w.Write(JsonResp)
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
		} else {

			for header, values := range response.Header {
				for _, value := range values {
					w.Header().Add(header, value)
				}
			}
			w.WriteHeader(response.StatusCode)
			defer response.Body.Close()
			io.Copy(w, response.Body)
		}

	})
}

// GetDOH_ENDPOINT 从安全存储中获取DNS-over-HTTPS (DOH)的服务器端点列表。
// 该函数不接受参数，返回一个字符串切片，包含一个或多个DOH服务器的URL。
// 如果无法从存储中成功获取数据，或数据格式不正确，则返回空的字符串切片。
func GetDOH_ENDPOINT() ([]string, error) {
	var store, err = secretstore.Open("DNS-aggregators-golang-fastly-compute")
	if err != nil {
		log.Println(err)
		return []string{}, err
	}
	s, err := store.Get("DOH_ENDPOINT")
	if err != nil {
		log.Println(err)
		return []string{}, err
	}
	v, err := s.Plaintext()
	if err != nil {
		log.Println(err)
		return []string{}, err
	}
	str := string(v)

	if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {

		//json
		var arr []string
		err = json.Unmarshal([]byte(str), &arr)
		if err != nil {
			log.Println(err)
			return []string{}, err
		}
		return arr, nil
	} else {
		return []string{str}, nil
	}

}

// GetDOH_MINTTL 从加密存储中获取 DNS-over-HTTPS (DOH) 的最小TTL值。
// 该函数不接受参数，返回值为解析后的整型 TTL 值以及可能出现的错误。
func GetDOH_MINTTL() (int, error) {
	var store, err = secretstore.Open("DNS-aggregators-golang-fastly-compute")
	if err != nil {
		log.Println(err)
		return 0, err
	}
	s, err := store.Get("DOH_MINTTL")
	if err != nil {
		log.Println(err)
		return 0, err
	}
	v, err := s.Plaintext()
	if err != nil {
		log.Println(err)
		return 0, err
	}
	str := string(v)

	return (strconv.Atoi(str))

}

// GetDOH_PATHNAME 函数用于从安全存储中获取 DOH（DNS over HTTPS）的路径名称。
// 该函数不接受参数，返回一个字符串和一个错误值。
// 返回的字符串是 DOH 路径名称，如果获取失败，则返回空字符串和相应的错误信息。
func GetDOH_PATHNAME() (string, error) {
	var store, err = secretstore.Open("DNS-aggregators-golang-fastly-compute")
	if err != nil {
		log.Println(err)
		return "", err
	}
	s, err := store.Get("DOH_PATHNAME")
	if err != nil {
		log.Println(err)
		return "", err
	}
	v, err := s.Plaintext()
	if err != nil {
		log.Println(err)
		return "", err
	}
	str := string(v)

	return (str), nil

}

// RandomShuffle 函数用于对指定类型的切片进行随机打乱。
//
// 参数:
// arr []T: 待打乱顺序的切片。
//
// 返回值:
// []T: 打乱顺序后的切片。
func RandomShuffle[T any](arr []T) []T {
	// 使用当前时间的纳秒级种子初始化随机数生成器，以确保每次运行结果都不同。
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// 使用 rand.Shuffle 函数来随机打乱切片的顺序。
	// 这个函数会传入切片的长度以及一个交换元素的函数。
	r.Shuffle(len(arr), func(i, j int) {
		// 交换函数通过交换 arr[i] 和 arr[j] 来打乱顺序。
		arr[i], arr[j] = arr[j], arr[i]
	})
	return arr
}

// Forwarded 创建并返回一个处理HTTP请求的Fastly中间件函数。
// 该中间件会添加一个"Forwarded"头部，用于指示请求被转发的信息。
//
// 返回值:
//
//	FastlyHttpMiddleWare: 一个函数，接受一个fsthttp.Request指针和一个next函数，
//	                      并返回一个fsthttp.Response指针。
func Forwarded() FastlyHttpMiddleWare {
	return func(r *fsthttp.Request, next func(r *fsthttp.Request) *fsthttp.Response) *fsthttp.Response {
		var clienthost = r.RemoteAddr
		var address = r.Host
		if address == "" {
			address = r.URL.Host
		}
		var proto = r.URL.Scheme //"http"

		// if r.TLS != nil {
		// 	proto = "https"
		// }
		forwarded := fmt.Sprintf(
			"for=%s;by=%s;host=%s;proto=%s",
			clienthost, // 代理自己的标识或IP地址
			r.Host,     // 代理的标识
			address,    // 原始请求的目标主机名
			proto,      // 或者 "https" 根据实际协议
		)
		r.Header.Add("Forwarded", forwarded)
		return next(r)
	}
}

// ArrayFilter 是一个根据回调函数过滤数组元素的通用函数。
// 它接受一个类型参数 T 的数组 arr 和一个回调函数 callback，该回调函数对每个数组元素进行测试。
// 如果回调函数对某个元素返回 true，则该元素被加入到结果数组中。
//
// 参数：
// arr []T - 需要进行过滤的数组。
// callback func(T) bool - 用于测试每个元素的回调函数，如果该函数返回 true，则元素被包含在结果数组中。
//
// 返回值：
// []T - 过滤后由满足条件的元素组成的新数组。
func ArrayFilter[T any](arr []T, callback func(T) bool) []T {
	var result []T
	for _, v := range arr {
		if callback(v) {
			result = append(result, v)
		}
	}
	return result
}

// DohClientWithCache 在进行DNS查询时，加入了缓存机制。它会首先尝试从缓存中获取结果，如果缓存未命中，则向DOH服务器发送请求，并将结果存储到缓存中。
//
// 参数:
// msg: 代表DNS查询消息的dns.Msg对象。
// dohServerURL: DOH服务器的URL。
// requestheaders: 发送给DOH服务器的HTTP请求头。
//
// 返回值:
// *dns.Msg: 代表DNS响应消息的dns.Msg对象。
// map[string][]string: 返回的HTTP响应头。
// error: 如果过程中出现错误，则返回错误信息。
func DohClientWithCache(msg *dns.Msg, dohServerURL string, requestheaders map[string][]string) (*dns.Msg, map[string][]string, error) {

	responseheaders := http.Header(map[string][]string{"content-type": {"application/dns-message"}})
	msg.Id = 0
	body, err := msg.Pack()
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, nil, err
	}
	var cachekey = sha512.Sum512(append([]byte(dohServerURL), body...))
	var keyhex = hex.EncodeToString(cachekey[:])
	var minttl = 600
	dohminttl, err := GetDOH_MINTTL()
	if err != nil {
		log.Println(err)
	} else {
		minttl = dohminttl
	}
	var hit = true
	var ttlresult = minttl
	rc, err := simple.GetOrSet(cachekey[:], func() (simple.CacheEntry, error) {
		var entry = *new(simple.CacheEntry)
		hit = false
		var res, header, err = DohClient(msg, dohServerURL, requestheaders)
		if err != nil {
			return entry, err
		}
		body, err := res.Pack()
		if err != nil {
			log.Println(dohServerURL, err)
			return entry, err
		}
		responseheaders = header
		var minttl3 = minttl
		if len(res.Answer) > 0 {
			var minttl3 = int(math.Max(float64(minttl), float64(ArrayReduce(res.Answer, res.Answer[0].Header().Ttl, func(acc uint32, v dns.RR) uint32 {
				return uint32(math.Min(float64(acc), float64(v.Header().Ttl)))
			}))))
			ttlresult = minttl3
		}

		return simple.CacheEntry{
			Body: bytes.NewReader(body),
			TTL:  time.Second * time.Duration(minttl3),
		}, nil
	})
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()

	var res = &dns.Msg{}
	buffer, err := io.ReadAll(rc)
	if err != nil {
		return nil, nil, err
	}
	err = res.Unpack(buffer)
	if err != nil {
		log.Println(dohServerURL, err)
		return nil, nil, err
	}
	ttlresult = minttl
	if len(res.Answer) > 0 {
		var minttl3 = int(math.Max(float64(minttl), float64(ArrayReduce(res.Answer, res.Answer[0].Header().Ttl, func(acc uint32, v dns.RR) uint32 {
			return uint32(math.Min(float64(acc), float64(v.Header().Ttl)))
		}))))
		ttlresult = minttl3
	}
	if hit {
		responseheaders.Add("Cache-Status", "FastlyComputeCache "+http.Header(requestheaders).Get("host")+";key="+keyhex+";hit;ttl="+strconv.Itoa(ttlresult))
	} else {
		responseheaders.Add("Cache-Status", "FastlyComputeCache "+http.Header(requestheaders).Get("host")+";key="+keyhex+";stored;ttl="+strconv.Itoa(ttlresult)+";fwd=miss;fwd-status=200")
	}
	responseheaders.Add("cache-control", "public,max-age="+fmt.Sprint(ttlresult)+",s-maxage="+fmt.Sprint(ttlresult))
	return res, responseheaders, nil
}
