package apidGatewayTrace

import (
	"github.com/apid/apid-core"
	"github.com/apigee-labs/transicator/common"
)

const (
	APIGEE_SYNC_EVENT        = "ApigeeSync"
	TRACESIGNAL_PG_TABLENAME = "metadata.trace"
)

//initListener registers this listener for either a change or snapshot event
func (h *apigeeSyncHandler) initListener(services apid.Services) {
	services.Events().Listen(APIGEE_SYNC_EVENT, h)
}

//String returns the plugin name
func (h *apigeeSyncHandler) String() string {
	return pluginData.Name
}

//Handle delegates the processing of a changelist or snapshot to the appropriate handlers
func (h *apigeeSyncHandler) Handle(e apid.Event) {

	if changeSet, ok := e.(*common.ChangeList); ok {
		h.processChangeList(changeSet)
	} else if snapData, ok := e.(*common.Snapshot); ok {
		h.processSnapshot(snapData)
	} else {
		log.Debugf("Received invalid event. Ignoring. %v", e)
	}
}

//processSnapshot assumes that all rows have already been inserted by apidApigeeSync plugin, and merely updates the db
//version.  It also calls the idempotent InitAPI method of it's apiManager
func (h *apigeeSyncHandler) processSnapshot(snapshot *common.Snapshot) {

	log.Debugf("Snapshot received. Switching to DB version: %s", snapshot.SnapshotInfo)

	h.dbMan.setDbVersion(snapshot.SnapshotInfo)

	//InitAPI is idempotent
	h.apiMan.InitAPI()
	log.Debug("Snapshot processed")
}

//processChangeList notifies the API implementation of a possible change.  Only Insert and Delete operations are
//supported for trace signals
func (h *apigeeSyncHandler) processChangeList(changes *common.ChangeList) {

	log.Debugf("Processing changes")
	// changes have been applied to DB
	for _, change := range changes.Changes {
		switch change.Table {
		case TRACESIGNAL_PG_TABLENAME:
			switch change.Operation {
			case common.Insert:
				h.apiMan.notifyChange(true)
			case common.Delete:
				h.apiMan.notifyChange(true)
			case common.Update:
				log.Errorf("Update operation on table %s not supported", TRACESIGNAL_PG_TABLENAME)
			default:
				log.Errorf("unexpected operation: %s", change.Operation)
			}
		}
	}
}
