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

const (
	API_ERR_BAD_BLOCK 			= iota + 1
	API_ERR_INTERNAL
	blobStoreUri 				= "/blobs"
	configBearerToken       	= "apigeesync_bearer_token"
	configBlobServerBaseURI     = "apigeesync_blob_server_base"
	maxIdleConnsPerHost         = 50
	httpTimeout                 = time.Minute
)

func (a *apiManager) InitAPI() {
	if a.apiInitialized {
		return
	}
	services.API().HandleFunc(a.signalEndpoint, a.apiGetTraceSignalEndpoint).Methods("GET")
	services.API().HandleFunc(a.uploadEndpoint, a.apiUploadTraceDataEndpoint).Methods("POST")
	a.apiInitialized = true
	log.Debug("API endpoints initialized")
}

func (a *apiManager) notifyChange(arg interface{}) {
	a.newSignal <- arg
}
// TODO in fact this is already in https://github.com/apid/apid-core/blob/master/util/util.go#L37
func (a *apiManager) distributeEvents() {
	subscribers := make(map[chan getTraceSignalsResult]struct{})

	for {
		select {
		case _, ok := <-a.newSignal:
			if !ok {
				log.Errorf("Error encountered attempting to distribute trace events: %v", ok)
				return
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

	// If-None-Match is a csv of active debug session IDs
	ifNoneMatch := r.Header.Get("If-None-Match")
	log.Debugf("if-none-match: %s", ifNoneMatch)
	//TODO: What if "block" is given but "If-None-Match" is not?
	//TODO: from my understanding, if ifNoneMatch=="", we should return immediately and ignore this timeout

	// send unmodified if matches prior eTag and no timeout
	result, err := a.dbMan.getTraceSignals()
	//TODO: add err!=nil check and maybe return 500
	if err == nil && ifNoneMatch != ""{
		clientTraceSessionExistence := make(map[string]bool)
		apidTraceSessionExistence := make(map[string]bool)
		/* TODO: This func is too long. We may want a func like: "func needToSend (signalIds []string, clientTraceSessions []string) bool"
		* TODO: Call it like:
			if needToSend(signalIds, strings.Split(ifNoneMatch, ",")) {
				a.sendTheseTraceSignals(w, result)
			}
		*/
		for _, id := range strings.Split(ifNoneMatch, ",") {
			clientTraceSessionExistence[id] = true
		}
		//TODO: It looks like you're comparing two maps, and sendTheseTraceSignals if they mismatch, try reflect.DeepEqual
		for _, signal := range result.Signals {
			//append here for deletion check to come
			apidTraceSessionExistence[signal.Id] = true

			//check for new trace signals
			if (!clientTraceSessionExistence[signal.Id]) {
				a.sendTheseTraceSignals(w, result)
				return
			}
		}
		for id := range clientTraceSessionExistence {
			//check for deleted trace signal. If deleted, we should response to update the state
			if (!apidTraceSessionExistence[id]) {
				a.sendTheseTraceSignals(w, result)
				return
			}
		}

		if (timeout == 0) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// otherwise, subscribe to any new deployment changes
	var newDeploymentsChannel chan getTraceSignalsResult
	newDeploymentsChannel = make(chan getTraceSignalsResult, 1)
	a.addSubscriber <- newDeploymentsChannel

	log.Debug("Blocking request... Waiting for new trace signals.")

	select {
	case result := <-newDeploymentsChannel:
		if result.Err != nil {
			a.writeInternalError(w, "Database error")
		} else {
			a.sendTheseTraceSignals(w, result)
		}

	case <-time.After(time.Duration(timeout) * time.Second):
		a.removeSubscriber <- newDeploymentsChannel
		log.Debug("Blocking deployment request timed out.")
		if ifNoneMatch != "" {
			w.WriteHeader(http.StatusNotModified)
		} else {
			//TODO: from my understanding, if ifNoneMatch=="", we should return immediately without waiting for this timeout
			a.sendAllTraceSignals(w)
		}
	}
}
//TODO: add header content-type: application/json
func (a *apiManager) sendTheseTraceSignals(w http.ResponseWriter, result getTraceSignalsResult) {
	b, err := json.Marshal(result)
	if err != nil {
		log.Errorf("unable to marshal deployments: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		//TODO: call a.writeInternalError. json may succeed writing error,even if this json.Marshal failed.
		return
	}

	w.Write(b)

}

func (a *apiManager) sendAllTraceSignals(w http.ResponseWriter) {

	result, err := a.dbMan.getTraceSignals()
	if err != nil {
		a.writeInternalError(w, "Database error")
		return
	}

	b, err := json.Marshal(result)
	if err != nil {
		log.Errorf("unable to marshal deployments: %v", err)
		//TODO: call a.writeInternalError. json may succeed writing error, even if this json.Marshal failed.
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func (a *apiManager) apiUploadTraceDataEndpoint (w http.ResponseWriter, r *http.Request) {
	//TODO defer r.Body.Close()
	// initialize tracker client
	//TODO: https://golang.org/pkg/net/http/ : Clients and Transports are safe for concurrent use by multiple goroutines and for efficiency should only be created once and re-used.

	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
		},
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			//TODO: not sure if we should use Header.Add. In hybrid, this "Authorization' header may be also used by customer firewall/proxy. Not sure.
			req.Header.Set("Authorization", getBearerToken())
			return nil
		},
	}
	blobMetadata := blobCreationMetadata{}
	sessionId := r.Header.Get("X-Apigee-Debug-ID")
	//TODO: strings.Split is called twice here. We can reduce it to 1.
	if sessionId != "" && (len(strings.Split(sessionId, "__")) == 5){
		sessionIdComponents := strings.Split(sessionId, "__")
		blobMetadata.Customer = sessionIdComponents[0]
		blobMetadata.Organization = sessionIdComponents[0]
		blobMetadata.Environment = sessionIdComponents[1]
		blobMetadata.Tags = []string {sessionIdComponents[4], sessionId}
	} else {
		//TODO: use http.statusXXX constant
		a.writeError(w, 400, 400, fmt.Sprintf("Bad value for required header X-Apigee-Debug-ID: %s", sessionId))
		return
	}

	s, err := getSignedURL(httpClient, blobMetadata, config.GetString(configBlobServerBaseURI))
	if err != nil {
		w.WriteHeader(500)
	} else {
		res, err := uploadToBlobstore(httpClient, s, r.Body)
		if err != nil {
			//TODO: This err seems to be apid error, should return 500 and hide 401 from clients
			w.WriteHeader(401)

		} else {
			w.WriteHeader(res.StatusCode)
			w.Write([]byte("Successfully uploaded trace to blobstore"))
		}
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
		//TODO: we can write a []byte("json marshal error") in case json error
		log.Errorf("unable to marshal errorResponse: %v", err)
	} else {
		w.Write(bytes)
	}
	log.Debugf("sending %d error to client: %s", status, reason)
}

func (a *apiManager) writeInternalError(w http.ResponseWriter, err string) {
	a.writeError(w, http.StatusInternalServerError, API_ERR_INTERNAL, err)
}

func getSignedURL(client *http.Client, blobMetadata blobCreationMetadata, blobServerURL string) (string, error) {

	blobUri, err := url.Parse(blobServerURL)
	if err != nil {
		log.Panicf("bad url value for config %s: %s", blobUri, err)
	}

	blobUri.Path += blobStoreUri
	uri := blobUri.String()

	surl, err := postWithAuth(client, uri, blobMetadata)
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
	log.Debugf("%+v\n", res)
	if err != nil {
		log.Errorf("Invalid response from BlobServer for {%s} error: {%v}", uri, err)
		return "", err
	}


	return res.SignedUrl, nil
}

func uploadToBlobstore(client *http.Client, uriString string, data io.Reader) (*http.Response, error){
	req, err := http.NewRequest("PUT", uriString, data)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/octet-stream")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		res.Body.Close()
		//TODO: log.Error()
		return nil, fmt.Errorf("POST uri %s failed with status %d", uriString, res.StatusCode)
	}
	return res, nil
}

func postWithAuth(client *http.Client, uriString string, blobMetadata blobCreationMetadata) (io.ReadCloser, error) {

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