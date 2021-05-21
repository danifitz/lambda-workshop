package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/newrelic/go-agent/v3/integrations/logcontext/nrlogrusplugin"
	"github.com/newrelic/go-agent/v3/integrations/nrlambda"
	"github.com/newrelic/go-agent/v3/newrelic"
)

type getItemsRequest struct {
	SortBy                  string      `json:"SortBy"`
	SortOrder               string      `json:"SortOrder"`
	ItemsToGet              int         `json:"ItemsToGet"`
	DistributedTracePayload http.Header `json:"DistributedTracePayload"`
}

func handler(ctx context.Context, items getItemsRequest) (string, error) {
	// At this point, we're handling an invocation. Cold start is over; this code runs for each invocation.
	// We'd like to add a custom event, and a custom attribute. For that, we need the transaction.

	// Initialise the logger
	log := logrus.New()
	log.SetFormatter(nrlogrusplugin.ContextFormatter{})

	if txn := newrelic.FromContext(ctx); nil != txn {
		// This is an example of a custom event. `FROM MyGoEvent SELECT *` in New Relic will find this event.
		txn.Application().RecordCustomEvent("MyGoEvent", map[string]interface{}{
			"zip": "zap",
		})

		// Extract the headers from the payload and accept them
		hdrs := http.Header{}
		hdrs.Set(newrelic.DistributedTraceW3CTraceParentHeader, items.DistributedTracePayload.Get("Traceparent"))
		hdrs.Set(newrelic.DistributedTraceW3CTraceStateHeader, items.DistributedTracePayload.Get("Tracestate"))
		txn.AcceptDistributedTraceHeaders(newrelic.TransportOther, hdrs)
		log.WithContext(ctx).Info("Accepted Distributed Tracing Payload")

		// create a new segment (span) - do some simulated work
		segment := txn.StartSegment("goToSleep")
		log.WithContext(ctx).Info("Going to sleep....yawn")
		time.Sleep(1 * time.Second)
		log.WithContext(ctx).Info("Woke up....yawn")
		segment.End()

		// Insert the headers for passing onto the next function
		txn.InsertDistributedTraceHeaders(hdrs)

		// This attribute gets added to the normal AwsLambdaInvocation event
		txn.AddAttribute("ItemsToGet", items.ItemsToGet)
	}
	return "Success!", nil
}

func main() {
	// Here we are in cold start. Anything you do in main happens once.
	// In main, we initialize the agent.
	app, err := newrelic.NewApplication(nrlambda.ConfigOption())
	if nil != err {
		fmt.Println("error creating app (invalid config):", err)
	}

	// Then we start the lambda handler using `nrlambda` rather than `lambda`
	nrlambda.Start(handler, app)
}
