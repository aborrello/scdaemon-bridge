// +build linux

package main

import (
	"io"
	"io/ioutil"
	"os"
	"path"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLogger() *zap.SugaredLogger {

	if verboseLogging {
		return zap.New(zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(getOutputSink()),
			zap.NewAtomicLevelAt(zap.DebugLevel),
		)).Sugar()
	}

	return zap.NewNop().Sugar()

}

func getOutputSink() io.Writer {

	if verboseLogging {

		if dir, err := gpgconfLinux.GetDirectory("homedir"); err == nil {
			if handle, err := os.OpenFile(path.Join(dir, "scdaemon-bridge.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640); err == nil {
				return handle
			}
		}

		return os.Stderr

	}

	return ioutil.Discard

}
