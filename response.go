package request

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/bitly/go-simplejson"
)

// Response ...
type Response struct {
	*http.Response
	content []byte
}

// Json return Response Body as simplejson.Json
func (resp *Response) Json() (*simplejson.Json, error) {
	b, err := resp.DecompressedContent()
	if err != nil {
		return nil, err
	}
	return simplejson.NewJson(b)
}

// DecompressedContent return Response Body as []byte,
// opportunely decompressed.
func (resp *Response) DecompressedContent() (b []byte, err error) {
	if resp.content != nil {
		return resp.content, nil
	}

	reader, err := resp.DecompressedReader()
	if err != nil {
		return nil, err
	}

	defer reader.Close()
	if b, err = ioutil.ReadAll(reader); err != nil {
		return nil, err
	}

	resp.content = b
	return b, err
}

// IsGzipped tells whether the contee has gzip encoding.
func (resp *Response) IsGzipped() bool {
	contentEncoding := resp.Header.Get("Content-Encoding")
	contentEncoding = strings.TrimSpace(strings.ToLower(contentEncoding))
	return contentEncoding == "gzip"
}

// ReadAllRawBody is a shortcut for ioutil.ReadAll(resp.Body) that also closes the body.
func (resp *Response) ReadAllRawBody() ([]byte, error) {
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

var gzipReaderSyncPool *sync.Pool

func init() {
	gzipReaderSyncPool = &sync.Pool{}
}
func getGzipReader(src io.Reader) (reader io.ReadCloser, closer func(), err error) {
	var gzipReader *gzip.Reader
	if r := gzipReaderSyncPool.Get(); r != nil {
		gzipReader = r.(*gzip.Reader)
		err := gzipReader.Reset(src)
		if err != nil {
			return nil, nil, err
		}
	} else {
		gzipReader, err = gzip.NewReader(src)
		if err != nil {
			return nil, nil, err
		}
	}

	closer = func() {
		gzipReader.Close()
		gzipReaderSyncPool.Put(gzipReader)
	}

	return gzipReader, closer, nil
}

// DecompressedReaderFromPool returns a decompressing reader (if the content encoding is gzip or deflate);
// otherwise, it simply returns resp.Body;
// gzip readers are sourced from a pool.
func (resp *Response) DecompressedReaderFromPool() (reader io.ReadCloser, closer func(), err error) {

	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))) {
	case "gzip":
		reader, closer, err = getGzipReader(resp.Body)
		if err != nil {
			return nil, nil, err
		}
	case "deflate":
		if reader, err = zlib.NewReader(resp.Body); err != nil {
			return nil, nil, err
		}
		closer = func() {
			reader.Close()
		}
	default:
		reader = resp.Body
		closer = func() {
			reader.Close()
		}
	}

	return reader, closer, err
}

// DecompressedReader returns a decompressing reader (if the content encoding is gzip or deflate);
// otherwise, it simply returns resp.Body
func (resp *Response) DecompressedReader() (reader io.ReadCloser, err error) {

	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))) {
	case "gzip":
		if reader, err = gzip.NewReader(resp.Body); err != nil {
			return nil, err
		}
	case "deflate":
		if reader, err = zlib.NewReader(resp.Body); err != nil {
			return nil, err
		}
	default:
		reader = resp.Body
	}

	return reader, err
}

// Text return Response Body as string
func (resp *Response) Text() (string, error) {
	b, err := resp.DecompressedContent()
	s := string(b)
	return s, err
}

// OK check Response StatusCode < 400 ?
func (resp *Response) OK() bool {
	return resp.StatusCode < 400
}

// Ok check Response StatusCode < 400 ?
func (resp *Response) Ok() bool {
	return resp.OK()
}

// Reason return Response Status
func (resp *Response) Reason() string {
	return resp.Status
}

// URL return finally request url
func (resp *Response) URL() (*url.URL, error) {
	u := resp.Request.URL
	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound,
		http.StatusSeeOther, http.StatusTemporaryRedirect:
		location, err := resp.Location()
		if err != nil {
			return nil, err
		}
		u = u.ResolveReference(location)
	}
	return u, nil
}
