package logx

import (
	"io"
	"log"
	"os"
)

var (
	logFile     *os.File
	multiWriter io.Writer
)

func SetupLogFile() error {
	logFileName := "docker-build.log"
	var err error
	logFile, err = os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	multiWriter = io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	return nil
}

func GetMultiWriter() io.Writer {
	return multiWriter
}

func CloseLogFile() {
	if logFile != nil {
		logFile.Close()
	}
}
