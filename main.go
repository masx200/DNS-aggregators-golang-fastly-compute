// https://developer.fastly.com/solutions/examples/DNS-aggregators-golang-fastly-compute/
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
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
	// "fmt"
	// "io"
	"encoding/json"
	"log"

	"github.com/fastly/compute-sdk-go/fsthttp"
	// "github.com/quic-go/quic-go/http3"
)

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
type FastlyHttpMiddleWare = func(r *fsthttp.Request, next func() *fsthttp.Response) *fsthttp.Response

// DNSResult 结构体用于保存DNS查询的结果和可能发生的错误。
// 其中包含一个dns.Msg类型的Msg字段用于保存DNS消息体，以及一个error类型的Err字段用于保存查询过程中可能发生的错误。
type DNSResult struct {
	Msg *dns.Msg // DNS查询的完整响应消息体
	Err error    // DNS查询过程中发生的错误
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
	var errs = []error{}
	for range dohendpoions {
		var result, ok = <-waitchan
		if ok {
			if result.Err != nil {
				fmt.Println(result.Err)
				errs = append(errs, result.Err)
			} else {
				results = append(results, result.Msg)

			}
		}
	}
	if len(results) == 0 {
		return nil, errors.New("no dns result,all servers failure\n" + strings.Join(ArrayMap(errs, func(err error) string {

			return err.Error()
		}), "\n"))
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
	res.Answer = RandomShuffle(messages)
	return res, nil
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

// handleDNSRequest 处理DNS请求
// buf []byte: 包含DNS请求数据的字节切片
// dnsResolver func(msg *dns.Msg) (*dns.Msg, error): 一个函数，用于解析DNS请求并返回响应
// 返回值 *fsthttp.Response: 返回一个HTTP响应，包含DNS响应数据或错误信息
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

	headers := fsthttp.Header{"Content-Type": []string{"application/dns-message"}}
	var minttl = 600
	dohminttl, err := GetDOH_MINTTL()
	if err != nil {
		log.Println(err)
	} else {
		minttl = dohminttl
	}
	if len(res.Answer) > 0 {

		minttl = ArrayReduce(res.Answer, minttl, func(acc int, v dns.RR) int {
			return int(math.Max(float64(acc), float64(v.Header().Ttl)))
		})
		headers["cache-control"] = []string{"public,max-age=" + fmt.Sprint(minttl) + ",s-maxage=" + fmt.Sprint(minttl)}
	}
	for _, rr := range res.Answer {
		rr.Header().Ttl = uint32(math.Max(float64(minttl), float64(rr.Header().Ttl)))
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
		Header:     headers,
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

		// var notfound = false

		var response = CreateDOHMiddleWare(DnsResolver, func() string {

			var PATHNAME, err = GetDOH_PATHNAME()
			if err != nil {
				log.Println(err)
				return "/"
			} else {
				return PATHNAME
			}
			// return "/"
		})(r, func() *fsthttp.Response {
			// notfound = true
			return &fsthttp.Response{StatusCode: fsthttp.StatusNotFound}
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
		} else {
			w.WriteHeader(response.StatusCode)
			for header, values := range response.Header {
				for _, value := range values {
					w.Header().Add(header, value)
				}
			}
			defer response.Body.Close()
			io.Copy(w, response.Body)
		}

	})
}

// GetDOH_ENDPOINT 从安全存储中获取DNS-over-HTTPS (DOH)的服务器端点列表。
// 该函数不接受参数，返回一个字符串切片，包含一个或多个DOH服务器的URL。
// 如果无法从存储中成功获取数据，或数据格式不正确，则返回空的字符串切片。
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
