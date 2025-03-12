package kredis

import (
	"errors"

	"github.com/redis/go-redis/v9"
)

func PaginateScanCmd(scanFn func(cursor uint64) *redis.ScanCmd, callbackFn func(keys []string)) (err error) {
	var keys []string
	for cursor := uint64(0); ; {
		if keys, cursor, err = scanFn(cursor).Result(); err != nil {
			if errors.Is(err, redis.Nil) {
				err = nil
			}
			return err
		}

		callbackFn(keys)

		if cursor == 0 {
			return nil
		}
	}
}
