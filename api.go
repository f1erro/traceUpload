package apidGatewayTrace

import (
	"encoding/json"
	"fmt"
	"github.com/apid/apid-core/util"
	"github.com/pkg/errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	API_ERR_BAD_BLOCK = iota + 1
	API_ERR_DB_ERROR
	API_ERR_BAD_DATA_MARSHALL
	API_ERR_BAD_DEBUG_HEADER
	API_ERR_BLOBSTORE
	blobStoreUri               = "/blobs"
	configBearerToken          = "apigeesync_bearer_token"
	configBlobServerBaseURI    = "apigeesync_blob_server_base"
	maxIdleConnsPerHost        = 50
	httpTimeout                = time.Minute
	UPLOAD_TRACESESSION_HEADER = "X-Apigee-Debug-ID"
)

//InitAPI registers the two trace related endpoints, and starts a goroutine which assists in distributing
//events (new signals) in support of long polling
func (a *apiManager) InitAPI() {
	if a.apiInitialized {
		return
	}
	services.API().HandleFunc(a.signalEndpoint, a.apiGetTraceSignalEndpoint).Methods("GET")
	services.API().HandleFunc(a.uploadEndpoint, a.apiUploadTraceDataEndpoint).Methods("POST")
	a.apiInitialized = true
	go util.DistributeEvents(a.newSignal, a.addSubscriber)
	log.Debug("API endpoints initialized")
}

//notifyChange sends an object to the change notification channel used to kick of event distribution
func (a *apiManager) notifyChange(arg interface{}) {
	a.newSignal <- arg
}

//apiGetTraceSignalEndpoint is the API implementation for retrieving a list of trace sessions initiated via the MGMT API
func (a *apiManager) apiGetTraceSignalEndpoint(w http.ResponseWriter, r *http.Request) {
	b := r.URL.Query().Get("block")
	var timeout int
	if b != "" {
		var err error
		timeout, err = strconv.Atoi(b)
		if err != nil {
			writeError(w, http.StatusBadRequest, API_ERR_BAD_BLOCK, "bad block value, must be number of seconds")
			return
		}
	}
	log.Debugf("api timeout: %d", timeout)

	// If-None-Match is a csv of active debug session IDs
	ifNoneMatch := r.Header.Get("If-None-Match")
	log.Debugf("If-None-Match: %s", ifNoneMatch)

	if ifNoneMatch == "" {
		a.sendTraceSignals(nil, w)
		return
	}

	// send unmodified if matches prior eTag and no timeout
	result, err := a.dbMan.getTraceSignals()
	if err != nil {
		log.Errorf("%v", err)
		writeError(w, http.StatusInternalServerError, API_ERR_DB_ERROR, err.Error())
		return
	}

	if additionOrDeletionDetected(result, ifNoneMatch) {
		a.sendTraceSignals(result, w)
		return
	}

	if timeout == 0 {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	log.Debug("Blocking request... Waiting for new trace signals.")
	util.LongPolling(w, time.Duration(timeout)*time.Second, a.addSubscriber, a.sendTraceSignals, a.LongPollTimeoutHandler)

}

//LongPollTimeoutHandler is the simple callback to represent a StatusNotModified HTTP code in the event that
//the long polling block timeout, provided by the API caller, is reached
func (a *apiManager) LongPollTimeoutHandler(w http.ResponseWriter) {
	log.Debug("long-polling tracesignals request timed out.")
	w.WriteHeader(http.StatusNotModified)
}

//sendTraceSignals uses the database manager to retrieve the list of signals and write them to the response as JSON
func (a *apiManager) sendTraceSignals(signals interface{}, w http.ResponseWriter) {

	var result getTraceSignalsResult
	if signals != nil {
		result = signals.(getTraceSignalsResult)
	} else {
		var err error
		result, err = a.dbMan.getTraceSignals()
		if err != nil {
			writeError(w, http.StatusInternalServerError, API_ERR_DB_ERROR, err.Error())
			return
		}
	}

	b, err := json.Marshal(result)
	if err != nil {
		log.Errorf("unable to marshal trace signals: %v", err)
		writeError(w, http.StatusInternalServerError, API_ERR_BAD_DATA_MARSHALL, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

//apiUploadTraceDataEndpoint is the API Implementation for uploading the trace data for a single completed request
func (a *apiManager) apiUploadTraceDataEndpoint(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	sessionId := r.Header.Get(UPLOAD_TRACESESSION_HEADER)
	blobMetadata, err := createBlobMetadataFromSessionId(sessionId)
	if err != nil {
		writeError(w, http.StatusBadRequest, API_ERR_BAD_DEBUG_HEADER, err.Error())
		return
	}

	s, err := a.bsClient.getSignedURL(blobMetadata, config.GetString(configBlobServerBaseURI))
	if err != nil {
		err = errors.Wrap(err, "Unable to fetch signed upload URL")
		log.Errorf("%v", err)
		writeError(w, http.StatusInternalServerError, API_ERR_BLOBSTORE, "Unable fetch signed upload URL")
	} else {
		res, err := a.bsClient.uploadToBlobstore(s, r.Body)
		if err != nil {
			err = errors.Wrap(err, "Unable to use signed url for upload")
			log.Errorf("%v", err)
			writeError(w, http.StatusInternalServerError, API_ERR_BLOBSTORE, "Unable to use signed url for upload")
		} else {
			w.WriteHeader(res.StatusCode)
		}
	}
}

//writeError writes an error to the HTTP response, providing an error code which can be correlated with the enum
//at the top of this file, and a reason which is typically the output of an error's Error() method
func writeError(w http.ResponseWriter, status int, code int, reason string) {
	w.WriteHeader(status)
	e := errorResponse{
		ErrorCode: code,
		Reason:    reason,
	}
	bytes, err := json.Marshal(e)
	if err != nil {
		log.Errorf("unable to marshal errorResponse: %v", err)
	} else {
		w.Write(bytes)
	}
	log.Debugf("sending %d error to client: %s", status, reason)
}

//additionOrDeletionDetected compares what trace sessions are currently active on an MP and the actual state
//(active sessions) as represented by those which exist in the database.  An session which exists in the MP but not
//the database represents a deletion, whereas an entry which exists in the database but not the MP represents a new signal
func additionOrDeletionDetected(result getTraceSignalsResult, ifNoneMatch string) bool {
	clientTraceSessionExistence := make(map[string]bool)
	apidTraceSessionExistence := make(map[string]bool)
	for _, id := range strings.Split(ifNoneMatch, ",") {
		clientTraceSessionExistence[strings.TrimSpace(id)] = true
	}

	for _, signal := range result.Signals {
		//append here for deletion check to come
		apidTraceSessionExistence[signal.Id] = true

		//check for new trace signals
		if !clientTraceSessionExistence[signal.Id] {
			return true
		}
	}
	return !reflect.DeepEqual(clientTraceSessionExistence, apidTraceSessionExistence)
}

//createBlobMetadataFromSessionId parses the canonical sessionId, which is an MP internal format, into a useable data
//structure for interacting with the blob creation API of the blobstore service
func createBlobMetadataFromSessionId(sessionId string) (blobCreationMetadata, error) {
	blobMetadata := blobCreationMetadata{}
	if sessionId != "" {
		sessionIdComponents := strings.Split(sessionId, "__")
		if len(strings.Split(sessionId, "__")) == 5 {
			blobMetadata.Customer = sessionIdComponents[0]
			blobMetadata.Organization = sessionIdComponents[0]
			blobMetadata.Environment = sessionIdComponents[1]
			blobMetadata.Tags = []string{sessionIdComponents[4], sessionId}
			return blobMetadata, nil
		}
	}
	return blobMetadata, fmt.Errorf("Bad value for required header X-Apigee-Debug-ID: %s", sessionId)
}
