package addon

import (
	"bytes"
	"github.com/garyburd/redigo/redis"
	"github.com/lqqyt2423/go-mitmproxy/flow"
	"github.com/tidwall/sjson"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type DumperWithFilter struct {
	Base
	level int // 0: header 1: header + body
	Out   io.Writer

	host, uri, contentType *regexp.Regexp
	RedisPool              *redis.Pool
}

func NewFilterDumper(host, uri, contentType, redisUri string) *DumperWithFilter {
	r := &DumperWithFilter{host: nil, uri: nil, contentType: nil, RedisPool: nil}
	if len(redisUri) > 0 {
		r.RedisPool = &redis.Pool{
			MaxIdle:     16,
			MaxActive:   0,
			IdleTimeout: 300,
			Dial: func() (redis.Conn, error) {
				return redis.Dial("tcp", uri)
			},
		}
	} else {
		out, err := os.OpenFile("dump.data", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
		r.Out = out
	}
	if len(host) > 0 {
		if f, err := regexp.Compile(host); err == nil {
			r.host = f
		}
	}
	if len(uri) > 0 {
		if f, err := regexp.Compile(uri); err == nil {
			r.uri = f
		}
	}
	if len(contentType) > 0 {
		if f, err := regexp.Compile(contentType); err == nil {
			r.contentType = f
		}
	}
	return r
}

func (d *DumperWithFilter) Requestheaders(f *flow.Flow) {
	log := log.WithField("in", "DumperWithFilter")
	go func() {
		<-f.Done()
		if d.host != nil && !d.host.MatchString(f.Request.URL.Host) {
			return
		}
		if d.uri != nil && !d.uri.MatchString(f.Request.URL.RequestURI()) {
			return
		}

		req := map[string]interface{}{
			"method":  f.Request.Method,
			"uri":     f.Request.URL.RequestURI(),
			"proto":   f.Request.Proto,
			"headers": "",
			"body":    "",
		}
		rsp := map[string]interface{}{
			"proto":       f.Request.Proto,
			"status_code": 0,
			"headers":     "",
			"body":        "",
		}

		h := map[string]string{"Host": f.Request.URL.Host}

		if len(f.Request.Raw().TransferEncoding) > 0 {
			h["Transfer-Encoding"] = strings.Join(f.Request.Raw().TransferEncoding, ",")
		}
		if f.Request.Raw().Close {
			h["Connection"] = "close"
		}
		for k, v := range f.Request.Header {
			h[k] = v[0]
		}
		req["headers"] = h

		if d.level == 1 && f.Request.Body != nil && len(f.Request.Body) > 0 && CanPrint(f.Request.Body) {
			req["body"] = string(f.Request.Body)
		}

		if f.Response != nil {
			h := map[string]string{}
			rsp["status_code"] = f.Response.StatusCode
			rsp["status_text"] = http.StatusText(f.Response.StatusCode)

			for k, v := range f.Response.Header {
				h[k] = v[0]
			}
			rsp["headers"] = h
			if d.contentType != nil && !d.contentType.MatchString(f.Response.Header.Get("Content-Encoding")) {
				return
			}

			if d.level == 1 && f.Response.Body != nil && len(f.Response.Body) > 0 {
				body, _ := f.Response.DecodedBody()
				if len(body) > 0 && CanPrint(body) {
					rsp["body"] = string(body)
				}
			}
		}
		msg := ""
		msg, _ = sjson.Set(msg, "req", req)
		msg, _ = sjson.Set(msg, "rsp", rsp)
		if d.RedisPool != nil {
			d.RedisPool.Get().Do("lpush", "http-message-queue", msg)
		} else {
			buf := bytes.NewBuffer(make([]byte, 0))
			buf.WriteString(msg)
			_, err := d.Out.Write(buf.Bytes())
			if err != nil {
				log.Error(err)
			}
		}
	}()
}
