package apidGatewayTrace

import (
	"github.com/apid/apid-core"
)

const (
	TRACESIGNAL_DB_QUERY    = `SELECT id, uri, method FROM metadata_trace;`
)

func (dbc *dbManager) setDbVersion(version string) {
	db, err := dbc.data.DBVersion(version)
	if err != nil {
		log.Panicf("Unable to access database: %v", err)
	}
	dbc.dbMux.Lock()
	dbc.db = db
	dbc.dbMux.Unlock()
}

func (dbc *dbManager) getDb() apid.DB {
	dbc.dbMux.RLock()
	defer dbc.dbMux.RUnlock()
	return dbc.db
}

func (dbc *dbManager) initDb() error {
	return nil
}

func (dbc *dbManager) getTraceSignals() (result getTraceSignalsResult, err error) {

	var signals []traceSignal

	rows, err := dbc.getDb().Query(TRACESIGNAL_DB_QUERY)
	if err != nil {
		log.Errorf("DB Query \"%s\" failed %v", TRACESIGNAL_DB_QUERY, err)
		return
	}
	defer rows.Close()
	//TODO: I added a new feature in https://github.com/apid/apid-core/pull/27 You can try it once it's merged :-)
	for rows.Next() {
		var id, uri, method string
		err = rows.Scan(&id, &uri, &method)
		if err != nil {
			return getTraceSignalsResult{Err: err}, err
		}
		signals = append(signals, traceSignal{Id: id, Uri: uri, Method: method})
	}

	result.Signals = signals
	log.Debugf("Trace commands %v", signals)
	return
}
