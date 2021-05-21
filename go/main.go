package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/newrelic/go-agent/v3/integrations/logcontext/nrlogrusplugin"
	"github.com/newrelic/go-agent/v3/integrations/nrawssdk-v1"
	"github.com/newrelic/go-agent/v3/integrations/nrlambda"
	"github.com/newrelic/go-agent/v3/newrelic"
)

type getItemsRequest struct {
	SortBy                  string
	SortOrder               string
	ItemsToGet              int
	DistributedTracePayload http.Header
}

func handler(ctx context.Context) (string, error) {
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

		// invoke another AWS Lambda function
		sess := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
		// Instrument the session Handlers with New Relic AWS SDK integration
		nrawssdk.InstrumentHandlers(&sess.Handlers)

		client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

		log.WithContext(ctx).Info("Inserting Distributed Trace headers for context propagation")
		hdrs := http.Header{}
		txn.InsertDistributedTraceHeaders(hdrs)
		log.WithContext(ctx).Info(hdrs)
		request := getItemsRequest{"time", "descending", 10, hdrs}

		payload, err := json.Marshal(request)
		if err != nil {
			fmt.Println("Error marshalling MyGetItemsFunction request")
			os.Exit(0)
		}

		log.WithContext(ctx).Info("Invoking lambda function: newrelic-example-go1")

		// invoke the lambda function
		req, resp := client.InvokeRequest(&lambda.InvokeInput{
			ClientContext:  aws.String("newrelic-example-go"),
			FunctionName:   aws.String("newrelic-example-go1"),
			InvocationType: aws.String("Event"),
			LogType:        aws.String("Tail"),
			Payload:        payload,
		})

		// Add txn to http.Request's context
		req.HTTPRequest = newrelic.RequestWithTransactionContext(req.HTTPRequest, txn)

		// send the API Call request
		error := req.Send()
		if error == nil { // resp is now filled
			log.WithContext(ctx).Info("Request Complete, Got StatusCode: " + strconv.FormatInt(*resp.StatusCode, 10))
		}

		// This attribute gets added to the normal AwsLambdaInvocation event
		txn.AddAttribute("customAttribute", "customAttributeValue")
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
