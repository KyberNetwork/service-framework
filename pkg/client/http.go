package client

import (
	"github.com/KyberNetwork/kutils"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HttpCfg is hotcfg for a resty http client. On update, it
// creates a new resty client with the new config
// as well as instruments the client for metrics and tracing.
type HttpCfg struct {
	kutils.HttpCfg `mapstructure:",squash"`
	C              *resty.Client
}

func (*HttpCfg) OnUpdate(_, new *HttpCfg) {
	new.C = new.NewRestyClient()
	if tracer.Provider() != nil {
		new.C.SetTransport(otelhttp.NewTransport(new.C.GetClient().Transport))
	}
}
