// +build windows

package main

import (
	"io"
	"io/ioutil"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLogger() *zap.SugaredLogger {

	if verboseLogging {
		return zap.New(zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.Lock(os.Stderr),
			zap.NewAtomicLevelAt(zap.DebugLevel),
		)).Sugar()
	}

	return zap.NewNop().Sugar()

}

func getOutputSink() io.Writer {
	return ioutil.Discard
}
