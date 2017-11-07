package apidGatewayTrace

import (
	"github.com/stretchr/testify/mock"
	//"database/sql"
	//"time"
	//"github.com/apid/apid-core"
	"io"
	"net/http"
)

/* Mock API Manager */
type mockApiManager struct {
	mock.Mock
	apiManagerInterface
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

func (m *mockDbManager) initDb() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockDbManager) getTraceSignals() (getTraceSignalsResult, error) {
	args := m.Called()
	return args.Get(0).(getTraceSignalsResult), args.Error(1)
}

/* Mock Blobstore client */
type mockBlobstoreClient struct {
	mock.Mock
	blobstoreClientInterface
}

func (bc *mockBlobstoreClient) getSignedURL(blobMetadata blobCreationMetadata, blobServerURL string) (string, error) {
	args := bc.Called(blobMetadata, blobServerURL)
	return args.String(0), args.Error(1)
}

func (bc *mockBlobstoreClient) postWithAuth(uriString string, blobMetadata blobCreationMetadata) (io.ReadCloser, error) {
	args := bc.Called(uriString, blobMetadata)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (bc *mockBlobstoreClient) uploadToBlobstore(uriString string, data io.Reader) (*http.Response, error) {
	args := bc.Called(uriString, data)
	return args.Get(0).(*http.Response), args.Error(1)

}
