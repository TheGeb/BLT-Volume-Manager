package web

import (
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
)

func s3LogFn() func(op, bucket, key string, dur time.Duration, err error) {
	return applog.S3Call
}

func logInfo(event string) {
	applog.Info(event)
}

func logError(event string, err error) {
	applog.Error(event, err)
}
