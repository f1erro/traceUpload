package apidGatewayTrace

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	//"github.com/apigee-labs/transicator/common"
	"github.com/apid/apid-core"
	//"github.com/apid/apid-core/factory"
	"io/ioutil"
	"strconv"
	"sync"
)

var _ = Describe("DBManager", func() {

	Context("General db I/O functionality", func() {

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

		It("should set db correctly", func() {
			dbShouldEqual, err := dbMan.data.DBVersion(dataTestTempDir)
			Expect(err).To(Succeed())
			dbMan.setDbVersion(dataTestTempDir)
			Expect(dbShouldEqual).To(Equal(dbMan.getDb()))

		})

		It("should fetch data", func() {
			result, err := dbMan.getTraceSignals()
			Expect(err).To(Succeed())
			Expect(result.Err).To(Succeed())
			Expect(result.Signals).ToNot(BeNil())
			for index, signal := range result.Signals {
				Expect(signal.Id).To(Equal(strconv.Itoa(index)))
				Expect(signal.Uri).To(Equal("uri" + strconv.Itoa(index)))
			}
		})
	})

})

func setupTestDb(db apid.DB) {
	bytes, err := ioutil.ReadFile(fileDataTest)
	Expect(err).Should(Succeed())
	query := string(bytes)
	log.Debug(query)
	_, err = db.Exec(query)
	Expect(err).Should(Succeed())
}
