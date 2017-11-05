package apidGatewayTrace

import (
  "sync"
  "github.com/apid/apid-core"
)

var (
  log              apid.LogService   // set in initPlugin
  services         apid.Services
  config           apid.ConfigService

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
    bsClient: &blobstoreClient{},
    signalEndpoint: "/tracesignals",
    uploadEndpoint: "/uploadTrace",
    apiInitialized: false,
    newSignal: make(chan interface{}),
    addSubscriber: make(chan chan interface{}),
  }

  // initialize event handler
  eventHandler := &apigeeSyncHandler{
    dbMan:     dbMan,
    apiMan:    apiMan,
    closed:    false,
  }

  eventHandler.initListener(services)

  log.Debug("end init")

  return pluginData, nil
}

func init() {
  apid.RegisterPlugin(initPlugin, pluginData)
}

