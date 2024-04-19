package main

import (
	"bytes"
	// "context"
	// "crypto/sha512"
	// "embed"

	// _ "embed"
	"errors"
	"fmt"
	"io"
	"math"
	// "math/rand"
	// "mime"
	"net/http"
	// "net/url"
	// "path/filepath"
	// "strconv"
	"strings"
	// "time"

	// "github.com/fastly/compute-sdk-go/secretstore"

	// "io"
	// "os"
	"github.com/miekg/dns"
	// "net/http"
	// "io"
	// "net/http"
	// "net/url"
	// "encoding/base64"
	// "encoding/hex"

	// "fmt"
	// "io"
	// "encoding/json"
	"log"

	// "github.com/fastly/compute-sdk-go/cache/simple"
	"github.com/fastly/compute-sdk-go/fsthttp"
	// "github.com/quic-go/quic-go/http3"
)

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
	/* cname展平操作,还不能出现多余的cname记录 */
	if len(cnames) > 1 && len(others) == 0 {
		/* 为了解决浏览器对于cname记录重复的不兼容问题,需要把cname记录进行展平.还要把cname记录放在其他记录前面,不允许出现相同名字的cname记录. */
		cnames = UniqBy(cnames, func(rr dns.RR) string {
			return rr.Header().Name
		})
		cnames = []dns.RR{
			cnames[len(cnames)-1],
		}
		cnames[0].Header().Name = msg.Question[0].Name
	}
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
	/* 如果只有一个或者以上的cname记录,则把cname记录放在其他记录前面,不允许出现相同名字的cname记录.把其他记录的名字都改成同一个 */
	if len(cnames) >= 1 && len(others) > 0 {
		var recordname = cnames[len(cnames)-1].Header().Name
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
