package apidGatewayTrace

import (
	"github.com/apid/apid-core"
	"net/http"
	"sync"
)

var (
	log      apid.LogService // set in initPlugin
	services apid.Services
	config   apid.ConfigService
)

func initServices(s apid.Services) {
	services = s
	log = services.Log().ForModule("apidGatewayTrace")
	config = services.Config()
}

func initPlugin(s apid.Services) (apid.PluginData, error) {
	initServices(s)
	log.Debugf("Initializing %s", pluginData.Name)
	dbMan := &dbManager{
		data:  services.Data(),
		dbMux: sync.RWMutex{},
	}

	apiMan := &apiManager{
		dbMan: dbMan,
		bsClient: &blobstoreClient{
			httpClient: &http.Client{
				Transport: &http.Transport{
					MaxIdleConnsPerHost: maxIdleConnsPerHost,
				},
				Timeout: httpTimeout,
				CheckRedirect: func(req *http.Request, _ []*http.Request) error {
					req.Header.Set("Authorization", getBearerToken())
					return nil
				},
			},
		},
		signalEndpoint: "/tracesignals",
		uploadEndpoint: "/uploadTrace",
		apiInitialized: false,
		newSignal:      make(chan interface{}),
		addSubscriber:  make(chan chan interface{}),
	}

	// initialize event handler
	eventHandler := &apigeeSyncHandler{
		dbMan:  dbMan,
		apiMan: apiMan,
		closed: false,
	}

	eventHandler.initListener(services)

	log.Debug("end init")

	return pluginData, nil
}

func init() {
	apid.RegisterPlugin(initPlugin, pluginData)
}
