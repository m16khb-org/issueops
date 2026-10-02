package issueopsapp

import (
	"fmt"
	"io"
	"time"

	"issueops/internal/adapter/outbound/issueopsrecord"
	tracedomain "issueops/internal/domain/trace"
)

const issueOpsSlowSpanThreshold = 100 * time.Millisecond

func issueOpsRecordObserver(writer io.Writer) issueopsrecord.Observer {
	return issueopsrecord.NewJSONLineObserver(writer, issueOpsSlowSpanThreshold)
}

// issueOpsCLIRecordObserver binds the CLI process TRACEPARENT once. It is only
// composed for CLI runs; a span whose context already carries a request
// correlation keeps it. An invalid value warns without echoing it.
func issueOpsCLIRecordObserver(traceparent string, writer io.Writer) issueopsrecord.Observer {
	observer := issueOpsRecordObserver(writer)
	trace, valid := tracedomain.ParseTraceparent(traceparent)
	if !valid {
		if traceparent != "" {
			fmt.Fprintln(writer, "issueops: ignoring invalid TRACEPARENT")
		}
		return observer
	}
	return issueopsrecord.ObserverFunc(func(observation issueopsrecord.SpanObservation) {
		if observation.TraceID == "" {
			observation.TraceID, observation.ParentID, observation.TraceFlags = trace.TraceID, trace.ParentID, trace.Flags
		}
		observer.Observe(observation)
	})
}

func issueOpsRecordStore(
	scope string,
	observers ...issueopsrecord.Observer,
) issueopsrecord.Store {
	var observer issueopsrecord.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	return issueopsrecord.Store{Scope: scope, Observer: observer}
}
