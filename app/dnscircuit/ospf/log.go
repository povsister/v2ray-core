package ospf

import (
	"fmt"

	"github.com/v2fly/v2ray-core/v5/common/log"
)

func logDebug(format string, args ...interface{}) {
	newError(fmt.Sprintf(format, args...)).AtDebug().WriteToLog()
}

func logWarn(format string, args ...interface{}) {
	newError(fmt.Sprintf(format, args...)).AtWarning().WriteToLog()
}

func logErr(err error, format string, args ...interface{}) {
	newError(fmt.Sprintf(format, args...)).Base(err).AtError().WriteToLog()
}

func LogImportant(format string, args ...interface{}) {
	log.Record(&log.PrefixMessage{
		Prefix:  "OSPF",
		Content: fmt.Sprintf(format, args...),
	})
}
