package apidGatewayTrace

import (
	. "github.com/onsi/ginkgo"
	"testing"
	"github.com/apid/apid-core"
	"github.com/apid/apid-core/factory"
	. "github.com/onsi/gomega"
	"os"
)

const testTempDirBase = "./tmp/"
const fileDataTest = "test_data.sql"

var _ = BeforeSuite(func() {
	apid.Initialize(factory.DefaultServicesFactory())
	initServices(apid.AllServices())
	_ = os.MkdirAll(testTempDirBase, os.ModePerm)
})

func TestData(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ApidGatewayTrace Suite")
}
