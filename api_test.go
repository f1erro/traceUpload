package apidGatewayTrace

import (
	. "github.com/onsi/ginkgo"

	"net/http/httptest"

	"encoding/json"
	"errors"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var _ = Describe("API Implementation", func() {

	Context("Get Tracesignals API", func() {

		var dataTestTempDir string
		var dbMan *dbManager

		BeforeEach(func() {
			var err error
			dataTestTempDir, err = ioutil.TempDir(testTempDirBase, "sqlite3")
			Expect(err).NotTo(HaveOccurred())
			services.Config().Set("local_storage_path", dataTestTempDir)

			dbMan = &dbManager{
				data:  services.Data(),
				dbMux: sync.RWMutex{},
			}
			dbMan.setDbVersion(dataTestTempDir)
			setupTestDb(dbMan.getDb())
		})

		It("should send notifications to correct channel", func() {
			apiMan := apiManager{
				newSignal: make(chan interface{}),
			}

			go apiMan.notifyChange(true)
			select {
			case <-apiMan.newSignal:
				return
			case <-time.After(100 * time.Millisecond):
				Fail("APIManager failed to notify change channel of change")
			}
		})

		It("should error on bad block value", func() {
			apiMan := apiManager{}
			r := httptest.NewRequest("GET", "/tracesignals?block=abc", nil)
			w := httptest.NewRecorder()
			apiMan.apiGetTraceSignalEndpoint(w, r)
			Expect(w.Code).To(Equal(400))
		})

		It("should return all traces signals if If-None-Match header is not present", func() {
			r := httptest.NewRequest("GET", "/tracesignals?block=5", nil)
			w := httptest.NewRecorder()

			apiMan := apiManager{
				dbMan: dbMan,
			}

			apiMan.apiGetTraceSignalEndpoint(w, r)
			Expect(w.Code).To(Equal(200))
			res := w.Body
			signals := &getTraceSignalsResult{}
			err := json.Unmarshal(res.Bytes(), signals)
			Expect(err).To(Succeed())
			Expect(signals.Err).To(Succeed())
			Expect(signals.Signals).ToNot(BeNil())
			for index, signal := range signals.Signals {
				Expect(signal.Id).To(Equal(strconv.Itoa(index)))
				Expect(signal.Uri).To(Equal("uri" + strconv.Itoa(index)))
			}
		})

		It("should detect the deletion of a trace signal", func() {
			r := httptest.NewRequest("GET", "/tracesignals?block=5", nil)
			w := httptest.NewRecorder()
			r.Header.Add("If-None-Match", "0, 1, 2, 3, 4")
			_, err := dbMan.db.Exec("DELETE from metadata_trace WHERE id='4'") //delete 4
			Expect(err).To(Succeed())
			apiMan := apiManager{
				dbMan: dbMan,
			}

			apiMan.apiGetTraceSignalEndpoint(w, r)
			Expect(w.Code).To(Equal(200))
			res := w.Body
			signals := &getTraceSignalsResult{}
			err = json.Unmarshal(res.Bytes(), signals)
			Expect(err).To(Succeed())
			Expect(signals.Err).To(Succeed())
			Expect(signals.Signals).ToNot(BeNil())
			Expect(len(signals.Signals)).To(Equal(4))
			for index, signal := range signals.Signals {
				Expect(signal.Id).To(Equal(strconv.Itoa(index)))
				Expect(signal.Uri).To(Equal("uri" + strconv.Itoa(index)))
			}

		})

		It("should detect the addition of a trace signal", func() {
			r := httptest.NewRequest("GET", "/tracesignals?block=5", nil)
			w := httptest.NewRecorder()
			r.Header.Add("If-None-Match", "2,3,4")
			apiMan := apiManager{
				dbMan: dbMan,
			}

			apiMan.apiGetTraceSignalEndpoint(w, r)
			Expect(w.Code).To(Equal(200))
			res := w.Body
			signals := &getTraceSignalsResult{}
			err := json.Unmarshal(res.Bytes(), signals)
			Expect(err).To(Succeed())
			Expect(signals.Err).To(Succeed())
			Expect(signals.Signals).ToNot(BeNil())
			Expect(len(signals.Signals)).To(Equal(5))
			for index, signal := range signals.Signals {
				Expect(signal.Id).To(Equal(strconv.Itoa(index)))
				Expect(signal.Uri).To(Equal("uri" + strconv.Itoa(index)))
			}

		})

		It("should return 304 if no changes detected and block param not provided", func() {
			r := httptest.NewRequest("GET", "/tracesignals", nil)
			w := httptest.NewRecorder()
			r.Header.Add("If-None-Match", "0,1,2,3,4")
			apiMan := apiManager{
				dbMan: dbMan,
			}

			apiMan.apiGetTraceSignalEndpoint(w, r)
			Expect(w.Code).To(Equal(304))
		})

		It("should return 304 after long polling for specified time", func() {
			r := httptest.NewRequest("GET", "/tracesignals?block=2", nil)
			w := httptest.NewRecorder()
			w.Code = 0
			r.Header.Add("If-None-Match", "0,1,2,3,4")
			apiMan := apiManager{
				dbMan:         dbMan,
				newSignal:     make(chan interface{}),
				addSubscriber: make(chan chan interface{}),
			}
			apiMan.InitAPI()
			go apiMan.apiGetTraceSignalEndpoint(w, r)
			<-time.After(500 * time.Millisecond)
			Expect(w.Code).To(Equal(0))
			<-time.After(2 * time.Second)
			Expect(w.Code).To(Equal(304))
		})

		It("should return all tracesignals after change is detected", func() {
			r := httptest.NewRequest("GET", "/tracesignals?block=2", nil)
			w := httptest.NewRecorder()
			w.Code = 0
			r.Header.Add("If-None-Match", "0,1,2,3,4")
			apiMan := apiManager{
				dbMan:          dbMan,
				newSignal:      make(chan interface{}),
				addSubscriber:  make(chan chan interface{}),
				apiInitialized: false,
			}
			apiMan.InitAPI()
			go apiMan.apiGetTraceSignalEndpoint(w, r)
			<-time.After(1 * time.Second)
			Expect(w.Code).To(Equal(0))                                                          //has not completed yet
			_, err := dbMan.db.Exec("INSERT into metadata_trace (id, uri) VALUES('5', 'uri5');") //delete 4
			Expect(err).To(Succeed())
			apiMan.notifyChange(nil)
			<-time.After(1 * time.Second)
			Expect(w.Code).To(Equal(200))
			res := w.Body
			signals := &getTraceSignalsResult{}
			err = json.Unmarshal(res.Bytes(), signals)
			Expect(err).To(Succeed())
			Expect(signals.Err).To(Succeed())
			Expect(signals.Signals).ToNot(BeNil())
			Expect(len(signals.Signals)).To(Equal(6))
			for index, signal := range signals.Signals {
				Expect(signal.Id).To(Equal(strconv.Itoa(index)))
				Expect(signal.Uri).To(Equal("uri" + strconv.Itoa(index)))
			}
		})
	})

	Context("Upload Tracesignals API", func() {
		var mockBsClient mockBlobstoreClient
		BeforeEach(func() {
			mockBsClient = mockBlobstoreClient{}
		})
		It("should return 400 if debug session header is missing", func() {
			r := httptest.NewRequest("POST", "/uploadTrace", nil)
			w := httptest.NewRecorder()
			apiMan := apiManager{}
			apiMan.apiUploadTraceDataEndpoint(w, r)
			Expect(w.Code).To(Equal(400))

		})

		It("should return 400 if debug session header is invalid", func() {
			r := httptest.NewRequest("POST", "/uploadTrace", nil)
			r.Header.Add(UPLOAD_TRACESESSION_HEADER, "invalid")
			w := httptest.NewRecorder()
			apiMan := apiManager{}
			apiMan.apiUploadTraceDataEndpoint(w, r)
			Expect(w.Code).To(Equal(400))

		})

		It("should return 500 if cannot get signed upload url", func() {
			r := httptest.NewRequest("POST", "/uploadTrace", nil)
			r.Header.Add(UPLOAD_TRACESESSION_HEADER, "org__env__app__rev__testID")
			w := httptest.NewRecorder()
			w.Code = 0
			apiMan := apiManager{
				bsClient: &mockBsClient,
			}
			mockBsClient.On("getSignedURL", mock.AnythingOfType("blobCreationMetadata"), config.GetString(configBlobServerBaseURI)).Return("", errors.New("mock bsClient err: can't get url"))
			apiMan.apiUploadTraceDataEndpoint(w, r)
			Expect(w.Code).To(Equal(500))

		})

		It("should return 500 if cannot use upload URL", func() {
			r := httptest.NewRequest("POST", "/uploadTrace", nil)
			r.Header.Add(UPLOAD_TRACESESSION_HEADER, "org__env__app__rev__testID")
			w := httptest.NewRecorder()
			w.Code = 0
			apiMan := apiManager{
				bsClient: &mockBsClient,
			}
			mockBsClient.On("getSignedURL", mock.AnythingOfType("blobCreationMetadata"), config.GetString(configBlobServerBaseURI)).Return("testurl", nil)
			mockBsClient.On("uploadToBlobstore", "testurl", r.Body).Return(&http.Response{}, errors.New("mock bsClient err: can't upload"))

			apiMan.apiUploadTraceDataEndpoint(w, r)
			Expect(w.Code).To(Equal(500))

		})

		It("should return 200 on success, and more generally any response code from blobstore", func() {
			var body io.Reader = strings.NewReader("a trace")
			r := httptest.NewRequest("POST", "/uploadTrace", body)
			r.Header.Add(UPLOAD_TRACESESSION_HEADER, "org__env__app__rev__testID")
			w := httptest.NewRecorder()
			w.Code = 0
			apiMan := apiManager{
				bsClient: &mockBsClient,
			}
			mockBsClient.On("getSignedURL", mock.AnythingOfType("blobCreationMetadata"), config.GetString(configBlobServerBaseURI)).Return("testurl", nil)
			mockBsClient.On("uploadToBlobstore", "testurl", r.Body).Return(&http.Response{StatusCode: 200}, nil)

			apiMan.apiUploadTraceDataEndpoint(w, r)
			Expect(w.Code).To(Equal(200))

		})

		It("should copy response from blobstore", func() {
			r := httptest.NewRequest("POST", "/uploadTrace", nil)
			r.Header.Add(UPLOAD_TRACESESSION_HEADER, "org__env__app__rev__testID")
			w := httptest.NewRecorder()
			w.Code = 0
			apiMan := apiManager{
				bsClient: &mockBsClient,
			}
			mockBsClient.On("getSignedURL", mock.AnythingOfType("blobCreationMetadata"), config.GetString(configBlobServerBaseURI)).Return("testurl", nil)
			mockBsClient.On("uploadToBlobstore", "testurl", r.Body).Return(&http.Response{StatusCode: 401}, nil)

			apiMan.apiUploadTraceDataEndpoint(w, r)
			Expect(w.Code).To(Equal(401))
		})

	})

	Context("API Manager Util function tests", func() {
		It("should detect the deletion of a trace signal", func() {
			ifNoneMatchHeader := "1,2,7"
			traceSignalsResult := getTraceSignalsResult{}
			traceSignalsResult.Signals = []traceSignal{{Id: "1"}, {Id: "2"}, {Id: "7"}}
			Expect(additionOrDeletionDetected(traceSignalsResult, ifNoneMatchHeader)).To(BeFalse())

			//test white space in between commas is ignored
			ifNoneMatchHeader = "1, 2, 7"
			Expect(additionOrDeletionDetected(traceSignalsResult, ifNoneMatchHeader)).To(BeFalse())

			ifNoneMatchHeader = "1,2"
			Expect(additionOrDeletionDetected(traceSignalsResult, ifNoneMatchHeader)).To(BeTrue())

			ifNoneMatchHeader = "1,2,7,8"
			Expect(additionOrDeletionDetected(traceSignalsResult, ifNoneMatchHeader)).To(BeTrue())

			ifNoneMatchHeader = "2,7,8"
			Expect(additionOrDeletionDetected(traceSignalsResult, ifNoneMatchHeader)).To(BeTrue())

		})
	})
})
