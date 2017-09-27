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

func initPlugin(services apid.Services) (apid.PluginData, error) {
  log = services.Log().ForModule("apidGatewayTrace")
  config = services.Config()
  log.Debugf("Initializing %s", pluginData.Name)
  dbMan := &dbManager{
    data:  services.Data(),
    dbMux: sync.RWMutex{},
  }

  apiMan := &apiManager{
    dbMan: dbMan,
    signalEndpoint: "/tracesignals",
    uploadEndpoint: "/uploadTrace",
    apiInitialized: false,
    newSignal: make(chan interface{}),
    addSubscriber: make(chan chan getTraceSignalsResult),
    removeSubscriber: make(chan chan getTraceSignalsResult),
  }

  // initialize event handler
  eventHandler := &apigeeSyncHandler{
    dbMan:     dbMan,
    apiMan:    apiMan,
    closed:    false,
  }

  eventHandler.initListener(services)

  go apiMan.distributeEvents()

  log.Debug("end init")

  return pluginData, nil
}

func init() {
  apid.RegisterPlugin(initPlugin)
}
