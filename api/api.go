package api

import (
	"github.com/gorilla/mux"
	"github.com/rainbowmga/timetravel/service"
)

type API struct {
	records service.RecordService
}

func NewAPI(records service.RecordService) *API {
	return &API{records}
}

// generates v1 api routes
func (a *API) CreateRoutes(routes *mux.Router) {
	routes.Path("/records/{id}").HandlerFunc(a.GetRecords).Methods("GET")
	routes.Path("/records/{id}").HandlerFunc(a.PostRecords).Methods("POST")
}

// generates v2 api routes
func (a *API) CreateV2Routes(routes *mux.Router) {
    routes.Path("/records/{id}").HandlerFunc(a.GetRecordsV2).Methods("GET")
    routes.Path("/records/{id}").HandlerFunc(a.PostRecordsV2).Methods("POST")
    routes.Path("/records/{id}/version/{version}").HandlerFunc(a.GetRecordsWithVersion).Methods("GET")
    routes.Path("/records/{id}/versions").HandlerFunc(a.ListVersions).Methods("GET")
}