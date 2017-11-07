package apidGatewayTrace

import (
	"github.com/apid/apid-core"

	"github.com/pkg/errors"
)

const (
	TRACESIGNAL_DB_QUERY = `SELECT id, uri FROM metadata_trace;`
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

	signals := make([]traceSignal, 0)

	rows, err := dbc.getDb().Query(TRACESIGNAL_DB_QUERY)
	if err != nil {
		return result, errors.Wrapf(err, "DB Query \"%s\" failed %v", TRACESIGNAL_DB_QUERY)
	}
	defer rows.Close()
	for rows.Next() {
		var id, uri string
		err = rows.Scan(&id, &uri)
		if err != nil {
			err = errors.Wrap(err, "failed to scan row")
			return getTraceSignalsResult{Err: err}, err
		}
		signals = append(signals, traceSignal{Id: id, Uri: uri})
	}

	result.Signals = signals
	log.Debugf("Trace commands %v", signals)
	return
}
