package apidGatewayTrace

import (
	"sync"

	"github.com/30x/apid-core"
)

type dbManagerInterface interface {
	setDbVersion(string)
	initDb() error
	getTraceSignals() (result getTraceSignalsResult, err error)
}

type dbManager struct {
	data  apid.DataService
	db    apid.DB
	dbMux sync.RWMutex
}

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
	// nothing to do here yet
	return nil
}

func (dbc *dbManager) getTraceSignals() (result getTraceSignalsResult, err error) {

	var signals []traceSignal

	rows, err := dbc.getDb().Query(`
	SELECT id, uri, method FROM metadata_trace;`)
	if err != nil {
		log.Errorf("DB Query for metadata_trace failed %v", err)
		return
	}
	defer rows.Close()
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
