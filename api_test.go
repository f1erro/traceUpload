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
	"fmt"
)

var _ = Describe("API Implementation", func() {

	Context("unit tests", func() {

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
			fmt.Println("should send notifications to correct channel")
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
			fmt.Println("should error on bad block value")
			apiMan := apiManager{}
			r := httptest.NewRequest("GET", "/tracesignals?block=abc", nil)
			w := httptest.NewRecorder()
			apiMan.apiGetTraceSignalEndpoint(w, r)
			Expect(w.Code).To(Equal(400))
		})

		It("should return all traces signals if If-None-Match header is not present", func() {
			fmt.Println("should return all traces signals if If-None-Match header is not present")
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

		It("should detect a deletion", func() {
			fmt.Println("should detect a deletion")
			r := httptest.NewRequest("GET", "/tracesignals?block=5", nil)
			w := httptest.NewRecorder()
			r.Header.Add("If-None-Match", "0, 1, 2, 3") //delete 4
			_, err := dbMan.db.Exec("DELETE from metadata_trace WHERE id='4'")
			Expect(err).To(Succeed())
			apiMan := apiManager{
				dbMan: dbMan,
			}

			apiMan.apiGetTraceSignalEndpoint(w, r)
			fmt.Println("here")

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
	})

})
