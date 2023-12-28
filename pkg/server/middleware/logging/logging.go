package logging

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"

	"github.com/KyberNetwork/service-framework/pkg/observe/klog"
)

func Logger() logging.LoggerFunc {
	return func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		f := make(map[string]any, len(fields)/2)
		for iter := logging.Fields(fields).Iterator(); iter.Next(); {
			k, v := iter.At()
			f[k] = v
		}
		log := klog.LoggerFromCtx(ctx).WithFields(f)

		switch lvl {
		case logging.LevelDebug:
			log.Debug(msg)
		case logging.LevelInfo:
			log.Info(msg)
		case logging.LevelWarn:
			log.Warn(msg)
		case logging.LevelError:
			log.Error(msg)
		default:
			log.Errorf("[unknown level %v] %s", lvl, msg)
		}
	}
}
