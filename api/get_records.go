package api

import (
	"encoding/json"
	"fmt"
 	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// GET /records/{id}
// GetRecord retrieves the record.
func (a *API) GetRecords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := mux.Vars(r)["id"]

	idNumber, err := strconv.ParseInt(id, 10, 32)

	if err != nil || idNumber <= 0 {
		err := writeError(w, "invalid id; id must be a positive number", http.StatusBadRequest)
		logError(err)
		return
	}

    record, err := a.records.GetRecord(ctx, int(idNumber))
	if err != nil {
		err := writeError(w, fmt.Sprintf("record of id %v does not exist", idNumber), http.StatusBadRequest)
		logError(err)
		return
	}

	err = writeJSON(w, record, http.StatusOK)
	logError(err)
}

// GET /records/{id} for v2 api
// GetRecord retrieves the record.
func (a *API) GetRecordsV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := mux.Vars(r)["id"]

	idNumber, err := strconv.ParseInt(id, 10, 32)

	if err != nil || idNumber <= 0 {
		err := writeError(w, "invalid id; id must be a positive number", http.StatusBadRequest)
		logError(err)
		return
	}

	record, err := a.records.GetLatestRecord(ctx, int(idNumber))
	if err != nil {
		err := writeError(w, fmt.Sprintf("record of id %v does not exist", idNumber), http.StatusBadRequest)
		logError(err)
		return
	}

	err = writeJSON(w, record, http.StatusOK)
	logError(err)
}

// GET /records/{id}/version/{version}
func (a *API) GetRecordsWithVersion(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	version := mux.Vars(r)["version"]

	idNumber, err := strconv.ParseInt(id, 10, 32)
	if err != nil || idNumber <= 0 {
		err := writeError(w, "invalid id; id must be a positive number", http.StatusBadRequest)
		logError(err)
		return
	}

	verNumber, err := strconv.ParseInt(version, 10, 32)
	if err != nil || verNumber <= 0 {
		err := writeError(w, "invalid version; version must be a positive number", http.StatusBadRequest)
		logError(err)
		return
	}

    record, err := a.records.GetRecordByVersion(r.Context(), int(idNumber), int(verNumber))
	if err != nil {
		err := writeError(w, fmt.Sprintf("record of id %v with version %v does not exist", idNumber, verNumber), http.StatusBadRequest)
		logError(err)
		return
	}

	err = writeJSON(w, record, http.StatusOK)
	logError(err)
}

// ListVersions retrieves all versions of a record.
func (a *API) ListVersions(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	idNumber, err := strconv.ParseInt(id, 10, 32)
	if err != nil || idNumber <= 0 {
		err := writeError(w, "invalid id; id must be a positive number", http.StatusBadRequest)
		logError(err)
		return
	}

    versions, err := a.records.ListAllVersions(r.Context(), int(idNumber))
    if err != nil {
        http.Error(w, "could not list versions", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(struct {
        ID       int   `json:"id"`
        Versions []int `json:"versions"`
    }{ID: int(idNumber), Versions: versions})
}