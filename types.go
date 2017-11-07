package apidGatewayTrace

import (
	"github.com/apid/apid-core"
	"io"
	"net/http"
	"sync"
)

//errorResponse is the json structure returned to clients in the event of an error
type errorResponse struct {
	ErrorCode int    `json:"errorCode"`
	Reason    string `json:"reason"`
}

//blobstoreClientInterface defines the methods needed for this plugin to interact with blobstore
type blobstoreClientInterface interface {
	getSignedURL(metadata blobCreationMetadata, blobServerURL string) (string, error)
	uploadToBlobstore(uriString string, data io.Reader) (*http.Response, error)
	postWithAuth(uriString string, blobMetadata blobCreationMetadata) (io.ReadCloser, error)
}

//blobstoreClient implements blobstoreClientInterface
type blobstoreClient struct {
	httpClient *http.Client
}

//blobCreationMetadata represents the metadata needed to create a blob in blobstore
type blobCreationMetadata struct {
	Customer     string   `json:"customer"`
	Environment  string   `json:"environment"`
	Organization string   `json:"organization"`
	Tags         []string `json:"tags"`
}

//blobServerResponse represents the data structure returned by either the creation or fetching of a blob
type blobServerResponse struct {
	Id                       string   `json:"id"`
	Kind                     string   `json:"kind"`
	Self                     string   `json:"self"`
	SignedUrl                string   `json:"signedurl"`
	SignedUrlExpiryTimestamp string   `json:"signedurlexpirytimestamp"`
	Tags                     []string `json:"tags"`
	Store                    string   `json:"store"`
	Organization             string   `json"organization"`
	ContentType              string   `json:"contentType"`
	Customer                 string   `json:"customer"`
}

//apiddSyncHandler is what listens for apid events
type apigeeSyncHandler struct {
	dbMan  dbManagerInterface
	apiMan apiManagerInterface
	closed bool
}

//apiManager provides the API implementations and hooks into apid-core long polling event distribution tooling
type apiManagerInterface interface {
	InitAPI()
	notifyChange(interface{})
}

//apiManager implements apiManagerInterface
type apiManager struct {
	signalEndpoint string
	uploadEndpoint string
	dbMan          dbManagerInterface
	bsClient       blobstoreClientInterface
	apiInitialized bool
	newSignal      chan interface{}
	addSubscriber  chan chan interface{}
}

//dbManagerInterface defines the necessary methods for using the shared apid sqlite database
type dbManagerInterface interface {
	setDbVersion(string)
	initDb() error
	getTraceSignals() (result getTraceSignalsResult, err error)
}

//dbManager implements dbManagerInterface
type dbManager struct {
	data  apid.DataService
	db    apid.DB
	dbMux sync.RWMutex
}

//traceSignal is the structure used to represent the instruction to create a trace signal to the MP
type traceSignal struct {
	Id  string `json:"id"`
	Uri string `json:"uri"`
}

//getTraceSignalsResult is the structure returned to the client representing the list of active traceSignals
type getTraceSignalsResult struct {
	Signals []traceSignal `json:"signals"`
	Err     error         `json:"error"`
}
