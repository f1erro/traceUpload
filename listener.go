package apidGatewayTrace

import (
	"github.com/apid/apid-core"
	"github.com/apigee-labs/transicator/common"
)

const (
	APIGEE_SYNC_EVENT     		 = "ApigeeSync"
	TRACESIGNAL_PG_TABLENAME     = "metadata.trace"

)

func (h *apigeeSyncHandler) initListener(services apid.Services) {
	services.Events().Listen(APIGEE_SYNC_EVENT, h)
}

func (h *apigeeSyncHandler) String() string {
	return pluginData.Name
}

func (h *apigeeSyncHandler) Handle(e apid.Event) {

	if changeSet, ok := e.(*common.ChangeList); ok {
		h.processChangeList(changeSet)
	} else if snapData, ok := e.(*common.Snapshot); ok {
		h.processSnapshot(snapData)
	} else {
		log.Debugf("Received invalid event. Ignoring. %v", e)
	}
}


//todo handle case where trace signal comes in during new snapshot (not boot). need to debounce
func (h *apigeeSyncHandler) processSnapshot(snapshot *common.Snapshot) {

	log.Debugf("Snapshot received. Switching to DB version: %s", snapshot.SnapshotInfo)

	h.dbMan.setDbVersion(snapshot.SnapshotInfo)

	//InitAPI is idempotent
	h.apiMan.InitAPI()
	log.Debug("Snapshot processed")
}

func (h *apigeeSyncHandler) processChangeList(changes *common.ChangeList) {

	log.Debugf("Processing changes")
	// changes have been applied to DB
	for _, change := range changes.Changes {
		switch change.Table {
		case TRACESIGNAL_PG_TABLENAME:
			h.apiMan.notifyChange(true)
			switch change.Operation {
			case common.Insert:
			case common.Delete:
			case common.Update:
				log.Errorf("Update operation on table %s not supported", TRACESIGNAL_PG_TABLENAME)
			default:
				log.Errorf("unexpected operation: %s", change.Operation)
			}
		}
	}
}









