package client

import (
	"github.com/KyberNetwork/kutils"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/KyberNetwork/service-framework/pkg/common"
)

// HttpCfg is hotcfg for a resty http client. On update, it
// creates a new resty client with the new config
// as well as instruments the client for metrics and tracing.
type HttpCfg struct {
	kutils.HttpCfg `mapstructure:",squash"`
	C              *resty.Client
}

func (*HttpCfg) OnUpdate(_, new *HttpCfg) {
	new.C = new.NewRestyClient().OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
		if len(r.Header.Values(common.HeaderXRequestId)) == 0 {
			if traceID, ok := common.TraceIdFromCtx(r.Context()); ok {
				r.Header.Set(common.HeaderXRequestId, traceID.String())
			}
		}
		if len(r.Header.Values(common.HeaderXClientId)) == 0 {
			r.Header.Set(common.HeaderXClientId, common.GetServiceClientId())
		}
		return nil
	})
	if tracer.Provider() != nil {
		new.C.SetTransport(otelhttp.NewTransport(new.C.GetClient().Transport))
	}
}
