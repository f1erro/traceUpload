package apidGatewayTrace

import (
	"github.com/stretchr/testify/mock"
	//"database/sql"
	//"time"
	//"github.com/apid/apid-core"
)

/* Mock API Manager */
type mockApiManager struct {
	mock.Mock
}

func (m *mockApiManager) InitAPI() {
	m.Called()
}

func (m *mockApiManager) notifyChange(change interface{}) {
	m.Called(change)
}


/* Mock DB Manager */
type mockDbManager struct {
	mock.Mock
}

func (m *mockDbManager) setDbVersion(version string) {
	m.Called(version)
}

func (m *mockDbManager) initDb() error{
	args := m.Called()
	return args.Error(0)
}

func (m *mockDbManager) getTraceSignals() (getTraceSignalsResult, error) {
	args := m.Called()
	return args.Get(0).(getTraceSignalsResult), args.Error(1)
}