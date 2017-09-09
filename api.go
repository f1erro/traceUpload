package apidGatewayTrace

import (
	"net/http"
	"encoding/json"
	"strconv"
	"time"
	"strings"
	"io/ioutil"
	"io"
	"fmt"
	"net/url"
	"bytes"
)

type errorResponse struct {
	ErrorCode int    `json:"errorCode"`
	Reason    string `json:"reason"`
}

const (
	API_ERR_BAD_BLOCK 		= iota + 1
	API_ERR_INTERNAL
	blobStoreUri 			= "/blobs"
	configBearerToken       	= "apigeesync_bearer_token"
	configBlobServerBaseURI         = "apigeesync_blob_server_base"
	maxIdleConnsPerHost              = 50
	httpTimeout                      = time.Minute
)

type apiManagerInterface interface {
	InitAPI()
	notifyChange(interface{})

}

type traceSignal struct {
	Id     string `json:"id"`
	Uri    string `json:"uri"`
	Method string `json:"method"`
}

type getTraceSignalsResult struct {
	Signals []traceSignal `json:"signals"`
	Err     error `json:"error"`
}

type apiManager struct {
	signalEndpoint string
	uploadEndpoint        string
	dbMan dbManagerInterface
	apiInitialized      bool
	newSignal  chan interface{}
	addSubscriber       chan chan getTraceSignalsResult
	removeSubscriber    chan chan getTraceSignalsResult

}

func (a *apiManager) InitAPI() {
	if a.apiInitialized {
		return
	}
	services.API().HandleFunc(a.signalEndpoint, a.apiGetTraceSignalEndpoint).Methods("GET")
	services.API().HandleFunc(a.uploadEndpoint, a.apiUploadTraceDataEndpoint).Methods("GET")
	a.apiInitialized = true
	log.Debug("API endpoints initialized")
}

func (a *apiManager) notifyChange(arg interface{}) {
	a.newSignal <- arg
}

func (a *apiManager) distributeEvents() {
	subscribers := make(map[chan getTraceSignalsResult]struct{})

	for {
		select {
		case _, ok := <-a.newSignal: //once we debounce, use different channel
			if !ok {
				return // todo: using this?
			}
			subs := subscribers
			subscribers = make(map[chan getTraceSignalsResult]struct{})
			go func() {
				traceSignals, _ := a.dbMan.getTraceSignals()
				log.Debugf("delivering trace signals to %d subscribers", len(subs))
				for subscriber := range subs {
					log.Debugf("delivering to: %v", subscriber)
					subscriber <- traceSignals
				}
			}()
		case subscriber := <-a.addSubscriber:
			log.Debugf("Add subscriber: %v", subscriber)
			subscribers[subscriber] = struct{}{}
		case subscriber := <-a.removeSubscriber:
			log.Debugf("Remove subscriber: %v", subscriber)
			delete(subscribers, subscriber)
		}
	}
}

func (a *apiManager) apiGetTraceSignalEndpoint (w http.ResponseWriter, r *http.Request) {
	b := r.URL.Query().Get("block")
	var timeout int
	if b != "" {
		var err error
		timeout, err = strconv.Atoi(b)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, API_ERR_BAD_BLOCK, "bad block value, must be number of seconds")
			return
		}
	}
	log.Debugf("api timeout: %d", timeout)

	// If If-None-Match is a csv of active debug
	ifNoneMatch := r.Header.Get("If-None-Match")
	log.Debugf("if-none-match: %s", ifNoneMatch)

	// send unmodified if matches prior eTag and no timeout
	result, err := a.dbMan.getTraceSignals()
	//ignore an error here
	if err == nil && ifNoneMatch != ""{
		existingTraceSessions := make(map[string]bool)
		newTraceSessions := make([]traceSignal, 0)
		for _, id := range strings.Split(ifNoneMatch, ",") {
			existingTraceSessions[id] = true
		}
		for _, signal := range result.Signals {
			if (!existingTraceSessions[signal.Id]) {
				newTraceSessions = append(newTraceSessions, signal)
			}
		}

		if len(newTraceSessions) > 0 {
			a.sendTraceSignals(w,  getTraceSignalsResult{Signals: newTraceSessions})
			return
		} else if (timeout == 0) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// otherwise, subscribe to any new deployment changes
	var newDeploymentsChannel chan getTraceSignalsResult
	newDeploymentsChannel = make(chan getTraceSignalsResult, 1)
	a.addSubscriber <- newDeploymentsChannel // getting blocked here

	log.Debug("Blocking request... Waiting for new trace signals.")

	select {
	case result := <-newDeploymentsChannel:
		if result.Err != nil {
			a.writeInternalError(w, "Database error")
		} else {
			a.sendTraceSignals(w, result)
		}

	case <-time.After(time.Duration(timeout) * time.Second):
		a.removeSubscriber <- newDeploymentsChannel
		log.Debug("Blocking deployment request timed out.")
		if ifNoneMatch != "" {
			w.WriteHeader(http.StatusNotModified)
		} else {
			result, err := a.dbMan.getTraceSignals()
			if err != nil {
				a.writeInternalError(w, "Database error")
			} else {
				a.sendTraceSignals(w, result)
			}
		}
	}
}

func (a *apiManager) sendTraceSignals(w http.ResponseWriter, result getTraceSignalsResult) {

	b, err := json.Marshal(result)
	if err != nil {
		log.Errorf("unable to marshal deployments: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func (a *apiManager) apiUploadTraceDataEndpoint (w http.ResponseWriter, r *http.Request) {
	// initialize tracker client
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
		},
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			req.Header.Set("Authorization", getBearerToken())
			return nil
		},
	}
	s, err := getSignedURL(httpClient, config.GetString(configBlobServerBaseURI))
	if err != nil {
		w.WriteHeader(401)
	} else {
		w.Write([]byte(s))
	}
}

func (a *apiManager) writeError(w http.ResponseWriter, status int, code int, reason string) {
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

func (a *apiManager) writeInternalError(w http.ResponseWriter, err string) {
	a.writeError(w, http.StatusInternalServerError, API_ERR_INTERNAL, err)
}

func getSignedURL(client *http.Client, blobServerURL string) (string, error) {

	blobUri, err := url.Parse(blobServerURL)
	if err != nil {
		log.Panicf("bad url value for config %s: %s", blobUri, err)
	}

	blobUri.Path += blobStoreUri
	uri := blobUri.String()

	surl, err := postWithAuth(client, uri, getTestBlobMetadata())
	if err != nil {
		log.Errorf("Unable to get signed URL from BlobServer %s: %v", uri, err)
		return "", err
	}
	defer surl.Close()

	body, err := ioutil.ReadAll(surl)
	if err != nil {
		log.Errorf("Invalid response from BlobServer for {%s} error: {%v}", uri, err)
		return "", err
	}
	res := blobServerResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		log.Errorf("Invalid response from BlobServer for {%s} error: {%v}", uri, err)
		return "", err
	}

	return res.SignedUrl, nil
}

func postWithAuth(client *http.Client, uriString string, blobMetadata createBlobMetadata) (io.ReadCloser, error) {

	b, err := json.Marshal(blobMetadata)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal blob metadata for blob %v", blobMetadata)
	}

	req, err := http.NewRequest("POST", uriString, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	// add Auth
	req.Header.Add("Authorization", getBearerToken())
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		res.Body.Close()
		return nil, fmt.Errorf("POST uri %s failed with status %d", uriString, res.StatusCode)
	}
	return res.Body, nil
}

func getBearerToken() string {
	return "Bearer " + config.GetString(configBearerToken)
}