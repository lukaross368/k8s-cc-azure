package loggers

import "log"

var Verbose bool

func LogVerbose(format string, v ...interface{}) {
	if Verbose {
		log.Printf(format, v...)
	}
}
