package apidGatewayTrace

import (
	. "github.com/onsi/ginkgo"
	//"github.com/apigee-labs/transicator/common"
	"time"
	"net/http/httptest"

	. "github.com/onsi/gomega"
	"io/ioutil"
	"sync"
	"encoding/json"
	"strconv"
)

var _ = Describe("API Implementation", func() {

	Context("API Manager Tests", func() {

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
				case <-time.After(100*time.Millisecond):
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
				dbMan: dbMan,
				newSignal: make(chan interface{}),
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
				dbMan: dbMan,
				newSignal: make(chan interface{}),
				addSubscriber: make(chan chan interface{}),
				apiInitialized: false,
			}
			apiMan.InitAPI()
			go apiMan.apiGetTraceSignalEndpoint(w, r)
			<-time.After(1 * time.Second)
			Expect(w.Code).To(Equal(0)) //has not completed yet
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
