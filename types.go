package apidGatewayTrace

import (
	"sync"
	"github.com/apid/apid-core"
	"net/http"
	"io"
)

type errorResponse struct {
	ErrorCode int    `json:"errorCode"`
	Reason    string `json:"reason"`
}

type blobstoreClientInterface interface {
  getSignedURL(metadata blobCreationMetadata, blobServerURL string) (string, error)
  uploadToBlobstore(uriString string, data io.Reader) (*http.Response, error)
  postWithAuth(uriString string, blobMetadata blobCreationMetadata) (io.ReadCloser, error)
}

type blobstoreClient struct {
	httpClient *http.Client
}

//blobstore types
type blobCreationMetadata struct {
	Customer    	string `json:"customer"`
	Environment 	string `json:"environment"`
	Organization 	string `json:"organization"`
	Tags 		[]string `json:"tags"`
}

type blobServerResponse struct {
	Id                       string `json:"id"`
	Kind                     string `json:"kind"`
	Self                     string `json:"self"`
	SignedUrl                string `json:"signedurl"`
	SignedUrlExpiryTimestamp string `json:"signedurlexpirytimestamp"`
	Tags			 []string `json:"tags"`
	Store 			 string `json:"store"`
	Organization		 string `json"organization"`
	ContentType		 string `json:"contentType"`
	Customer		 string `json:"customer"`
}

//listener types
type apigeeSyncHandler struct {
	dbMan     dbManagerInterface
	apiMan    apiManagerInterface
	closed    bool
}

//api implementation types
type apiManagerInterface interface {
	InitAPI()
	notifyChange(interface{})
}

type apiManager struct {
	signalEndpoint 		string
	uploadEndpoint      string
	dbMan 				dbManagerInterface
	bsClient			blobstoreClientInterface
	apiInitialized      bool
	newSignal  		    chan interface{}
	addSubscriber       chan chan interface{}

}

//data management types
type dbManagerInterface interface {
	setDbVersion(string)
	initDb() error
	getTraceSignals() (result getTraceSignalsResult, err error)
}

type dbManager struct {
	data  apid.DataService
	db    apid.DB
	dbMux sync.RWMutex
}

type traceSignal struct {
	Id     string `json:"id"`
	Uri    string `json:"uri"`
}

type getTraceSignalsResult struct {
	Signals []traceSignal `json:"signals"`
	Err     error `json:"error"`
}