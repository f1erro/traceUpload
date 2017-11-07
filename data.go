package apidGatewayTrace

import (
	"github.com/apid/apid-core"

	"github.com/pkg/errors"
)

const (
	TRACESIGNAL_DB_QUERY = `SELECT id, uri FROM metadata_trace;`
)

//setDbVersion updates the database version so that our database connection connects to the correct sqlite database
func (dbc *dbManager) setDbVersion(version string) {
	db, err := dbc.data.DBVersion(version)
	if err != nil {
		log.Panicf("Unable to access database: %v", err)
	}
	dbc.dbMux.Lock()
	dbc.db = db
	dbc.dbMux.Unlock()
}

//getDb is a mutex protected access method to the database client
func (dbc *dbManager) getDb() apid.DB {
	dbc.dbMux.RLock()
	defer dbc.dbMux.RUnlock()
	return dbc.db
}

//initDb currently does nothing, but is here as scaffolding in case database manipulation is desired or useful in the future
func (dbc *dbManager) initDb() error {
	return nil
}

//getTraceSignals issues a SQL query to retrieve all trace signals known to apid
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
