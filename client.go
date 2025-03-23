package requests

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"

	utls "github.com/refraction-networking/utls"
)

// 標準的なhttp.RoundTripperをラップするアダプター
type standardRoundTripperAdapter struct {
	tr http.RoundTripper
}

func (a *standardRoundTripperAdapter) RoundTrip(resp *Response) error {
	// リクエストオブジェクトを取得
	req := resp.Request()

	// 標準RoundTripperを使用してリクエストを送信
	httpResp, err := a.tr.RoundTrip(req)
	if err != nil {
		return err
	}

	// レスポンスをセット
	resp.response = httpResp

	return nil
}

// 標準的なio.ReadCloserをラップするダミーのConn実装
type dummyConn struct {
	body io.ReadCloser
	ctx  context.Context
}

func (d *dummyConn) CloseWithError(err error) error {
	return d.body.Close()
}

func (d *dummyConn) DoRequest(req *http.Request, headers []interface {
	Key() string
	Val() any
}) (*http.Response, context.Context, error) {
	return nil, d.ctx, errors.New("dummy connection does not support DoRequest")
}

func (d *dummyConn) CloseCtx() context.Context {
	return d.ctx
}

func (d *dummyConn) Stream() io.ReadWriteCloser {
	// 通常はWebSocketなどで使用するため、ダミー実装
	return nil
}

func (a *standardRoundTripperAdapter) closeConns() {
	// 標準トランスポートのCloseIdleConnsを呼び出す（可能な場合）
	if tr, ok := a.tr.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

// Connection Management
type Client struct {
	ctx          context.Context
	transport    requestRoundTripper // インターフェースを使用
	cnl          context.CancelFunc
	ClientOption ClientOption
	closed       bool
}

var defaultClient, _ = NewClient(context.TODO())

// New Connection Management
func NewClient(preCtx context.Context, options ...ClientOption) (*Client, error) {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	var option ClientOption
	if len(options) > 0 {
		option = options[0]
	}
	result := new(Client)
	result.ctx, result.cnl = context.WithCancel(preCtx)

	// カスタムTransportが指定されている場合はアダプターでラップ
	if option.Transport != nil {
		result.transport = &standardRoundTripperAdapter{tr: option.Transport}
	} else {
		// 従来通りの新しいTransportを作成
		trp := newRoundTripper(result.ctx)
		result.transport = trp
	}

	result.ClientOption = option
	if result.ClientOption.TlsConfig == nil {
		result.ClientOption.TlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(0),
		}
	}
	if result.ClientOption.UtlsConfig == nil {
		result.ClientOption.UtlsConfig = &utls.Config{
			InsecureSkipVerify:                 true,
			ClientSessionCache:                 utls.NewLRUClientSessionCache(0),
			InsecureSkipTimeVerify:             true,
			OmitEmptyPsk:                       true,
			PreferSkipResumptionOnNilExtension: true,
		}
	}
	//cookiesjar
	if !result.ClientOption.DisCookie {
		if result.ClientOption.Jar == nil {
			result.ClientOption.Jar = NewJar()
		}
	}
	var err error
	if result.ClientOption.Proxy != nil {
		_, err = parseProxy(result.ClientOption.Proxy)
	}
	return result, err
}

func (obj *Client) CloseConns() {
	obj.transport.closeConns()
}

// Close the client and cannot be used again after shutdown
func (obj *Client) Close() {
	obj.closed = true
	obj.CloseConns()
	obj.cnl()
}
