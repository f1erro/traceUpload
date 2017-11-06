package apidGatewayTrace

import (
	"github.com/apigee-labs/transicator/common"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Listener", func() {

	Context("ApigeeSync snapshot event", func() {

		It("listener should process a snapshot", func() {
			apiManager := new(mockApiManager)
			apiManager.On("InitAPI").Return()

			dbManager := new(mockDbManager)
			dbManager.On("setDbVersion", "testSnapshotId").Return()

			handler := apigeeSyncHandler{
				dbMan:  dbManager,
				apiMan: apiManager,
				closed: false,
			}
			handler.Handle(&common.Snapshot{SnapshotInfo: "testSnapshotId"})
			apiManager.AssertNumberOfCalls(GinkgoT(), "InitAPI", 1)
			dbManager.AssertNumberOfCalls(GinkgoT(), "setDbVersion", 1)

		})

		It("listener should process a changelist", func() {
			apiManager := new(mockApiManager)
			apiManager.On("notifyChange", true)
			handler := apigeeSyncHandler{
				dbMan:  nil,
				apiMan: apiManager,
				closed: false,
			}
			// test changelist with all operations.  Only Insert and Delete for trace table should trigger notifications
			handler.Handle(&common.ChangeList{Changes: []common.Change{
				{Table: TRACESIGNAL_PG_TABLENAME, Operation: common.Insert},
				{Table: TRACESIGNAL_PG_TABLENAME, Operation: common.Delete},
				{Table: "not.trace.metadata", Operation: common.Insert},
				{Table: TRACESIGNAL_PG_TABLENAME, Operation: common.Update}}})
			apiManager.AssertNumberOfCalls(GinkgoT(), "notifyChange", 2)
		})
	})

})
