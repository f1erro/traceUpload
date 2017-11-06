package apidGatewayTrace

import (
	"encoding/json"
	"fmt"
	"github.com/apid/apid-core/util"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	API_ERR_BAD_BLOCK = iota + 1
	API_ERR_INTERNAL
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

func (a *apiManager) notifyChange(arg interface{}) {
	a.newSignal <- arg
}

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
	log.Debugf("if-none-match: %s", ifNoneMatch)

	if ifNoneMatch == "" {
		a.sendTraceSignals(nil, w)
		return
	}

	// send unmodified if matches prior eTag and no timeout
	result, err := a.dbMan.getTraceSignals()
	if err != nil {

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

func (a *apiManager) LongPollTimeoutHandler(w http.ResponseWriter) {
	log.Debug("long-polling tracesignals request timed out.")
	w.WriteHeader(http.StatusNotModified)
}

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
		writeError(w, http.StatusInternalServerError, API_ERR_BAD_DATA_MARSHALL, "Unable to marshal trace signals")
		return
	}

	w.Write(b)
}

func (a *apiManager) apiUploadTraceDataEndpoint(w http.ResponseWriter, r *http.Request) {
	blobMetadata := blobCreationMetadata{}
	sessionId := r.Header.Get(UPLOAD_TRACESESSION_HEADER)
	if sessionId != "" && (len(strings.Split(sessionId, "__")) == 5) {
		sessionIdComponents := strings.Split(sessionId, "__")
		blobMetadata.Customer = sessionIdComponents[0]
		blobMetadata.Organization = sessionIdComponents[0]
		blobMetadata.Environment = sessionIdComponents[1]
		blobMetadata.Tags = []string{sessionIdComponents[4], sessionId}
	} else {
		writeError(w, 400, API_ERR_BAD_DEBUG_HEADER, fmt.Sprintf("Bad value for required header X-Apigee-Debug-ID: %s", sessionId))
		return
	}

	s, err := a.bsClient.getSignedURL(blobMetadata, config.GetString(configBlobServerBaseURI))
	if err != nil {
		log.Errorf("Unable to fetch signed upload URL: %v", err)
		writeError(w, http.StatusInternalServerError, API_ERR_BLOBSTORE, "Unable fetch signed upload URL")
	} else {
		res, err := a.bsClient.uploadToBlobstore(s, r.Body)
		if err != nil {
			log.Errorf("Unable to use signed url for upload: %v", err)
			writeError(w, http.StatusInternalServerError, API_ERR_BLOBSTORE, "Unable to use signed url for upload")
		} else {
			w.WriteHeader(res.StatusCode)
		}
	}
}

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
	for id := range clientTraceSessionExistence {
		//check for deleted trace signal. If deleted, we should response to update the state
		if !apidTraceSessionExistence[id] {
			return true
		}
	}

	return false
}
