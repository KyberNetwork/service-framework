package client

import (
	"net/http"

	"github.com/KyberNetwork/kutils"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type HttpCfg struct {
	kutils.HttpCfg `mapstructure:",squash"`
	C              *resty.Client
}

func (*HttpCfg) OnUpdate(_, new *HttpCfg) {
	if tracer.Provider() != nil {
		if new.HttpClient == nil {
			new.HttpClient = &http.Client{}
		}
		new.HttpClient.Transport = otelhttp.NewTransport(new.HttpClient.Transport)
	}
	new.C = new.NewRestyClient()
}
