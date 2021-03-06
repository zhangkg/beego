package httplib

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

var defaultUserAgent = "beegoServer"

func Get(url string) *BeegoHttpRequest {
	var req http.Request
	req.Method = "GET"
	req.Header = http.Header{}
	req.Header.Set("User-Agent", defaultUserAgent)
	return &BeegoHttpRequest{url, &req, map[string]string{}, false, 60 * time.Second, 60 * time.Second}
}

func Post(url string) *BeegoHttpRequest {
	var req http.Request
	req.Method = "POST"
	req.Header = http.Header{}
	req.Header.Set("User-Agent", defaultUserAgent)
	return &BeegoHttpRequest{url, &req, map[string]string{}, false, 60 * time.Second, 60 * time.Second}
}

func Put(url string) *BeegoHttpRequest {
	var req http.Request
	req.Method = "PUT"
	req.Header = http.Header{}
	req.Header.Set("User-Agent", defaultUserAgent)
	return &BeegoHttpRequest{url, &req, map[string]string{}, false, 60 * time.Second, 60 * time.Second}
}

func Delete(url string) *BeegoHttpRequest {
	var req http.Request
	req.Method = "DELETE"
	req.Header = http.Header{}
	req.Header.Set("User-Agent", defaultUserAgent)
	return &BeegoHttpRequest{url, &req, map[string]string{}, false, 60 * time.Second, 60 * time.Second}
}

func Head(url string) *BeegoHttpRequest {
	var req http.Request
	req.Method = "HEAD"
	req.Header = http.Header{}
	req.Header.Set("User-Agent", defaultUserAgent)
	return &BeegoHttpRequest{url, &req, map[string]string{}, false, 60 * time.Second, 60 * time.Second}
}

type BeegoHttpRequest struct {
	url              string
	req              *http.Request
	params           map[string]string
	showdebug        bool
	connectTimeout   time.Duration
	readWriteTimeout time.Duration
}

func (b *BeegoHttpRequest) Debug(isdebug bool) *BeegoHttpRequest {
	b.showdebug = isdebug
	return b
}

func (b *BeegoHttpRequest) SetTimeout(connectTimeout, readWriteTimeout time.Duration) *BeegoHttpRequest {
	b.connectTimeout = connectTimeout
	b.readWriteTimeout = readWriteTimeout
	return b
}

func (b *BeegoHttpRequest) Header(key, value string) *BeegoHttpRequest {
	b.req.Header.Set(key, value)
	return b
}

func (b *BeegoHttpRequest) Param(key, value string) *BeegoHttpRequest {
	b.params[key] = value
	return b
}

func (b *BeegoHttpRequest) Body(data interface{}) *BeegoHttpRequest {
	switch t := data.(type) {
	case string:
		bf := bytes.NewBufferString(t)
		b.req.Body = ioutil.NopCloser(bf)
		b.req.ContentLength = int64(len(t))
	case []byte:
		bf := bytes.NewBuffer(t)
		b.req.Body = ioutil.NopCloser(bf)
		b.req.ContentLength = int64(len(t))
	}
	return b
}

func (b *BeegoHttpRequest) getResponse() (*http.Response, error) {
	var paramBody string
	if len(b.params) > 0 {
		var buf bytes.Buffer
		for k, v := range b.params {
			buf.WriteString(url.QueryEscape(k))
			buf.WriteByte('=')
			buf.WriteString(url.QueryEscape(v))
			buf.WriteByte('&')
		}
		paramBody = buf.String()
		paramBody = paramBody[0 : len(paramBody)-1]
	}

	if b.req.Method == "GET" && len(paramBody) > 0 {
		if strings.Index(b.url, "?") != -1 {
			b.url += "&" + paramBody
		} else {
			b.url = b.url + "?" + paramBody
		}
	} else if b.req.Method == "POST" && b.req.Body == nil && len(paramBody) > 0 {
		b.Header("Content-Type", "application/x-www-form-urlencoded")
		b.Body(paramBody)
	}

	url, err := url.Parse(b.url)
	if url.Scheme == "" {
		b.url = "http://" + b.url
		url, err = url.Parse(b.url)
	}
	if err != nil {
		return nil, err
	}

	b.req.URL = url
	if b.showdebug {
		dump, err := httputil.DumpRequest(b.req, true)
		if err != nil {
			println(err.Error())
		}
		println(string(dump))
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: TimeoutDialer(b.connectTimeout, b.readWriteTimeout),
		},
	}
	resp, err := client.Do(b.req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (b *BeegoHttpRequest) String() (string, error) {
	data, err := b.Bytes()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (b *BeegoHttpRequest) Bytes() ([]byte, error) {
	resp, err := b.getResponse()
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (b *BeegoHttpRequest) ToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	resp, err := b.getResponse()
	if err != nil {
		return err
	}
	if resp.Body == nil {
		return nil
	}
	defer resp.Body.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func (b *BeegoHttpRequest) ToJson(v interface{}) error {
	data, err := b.Bytes()
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func (b *BeegoHttpRequest) ToXML(v interface{}) error {
	data, err := b.Bytes()
	if err != nil {
		return err
	}
	err = xml.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func (b *BeegoHttpRequest) Response() (*http.Response, error) {
	return b.getResponse()
}

func TimeoutDialer(cTimeout time.Duration, rwTimeout time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, cTimeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(rwTimeout))
		return conn, nil
	}
}
