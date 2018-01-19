package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"protos/daemon"

	"github.com/gorilla/mux"
)

const (
	CregisterResourceProvider = "registerResourceProvider"
)

var internalRoutes = routes{
	route{
		"getOwnResources",
		"GET",
		"/internal/resource",
		getOwnResources,
	},
	route{
		"createResource",
		"POST",
		"/internal/resource",
		createResource,
	},
	route{
		"deleteResource",
		"DELETE",
		"/internal/resource/{resourceID}",
		deleteResource,
	},
	route{
		CregisterResourceProvider,
		"POST",
		"/internal/provider",
		registerResourceProvider,
	},
	route{
		"deregisterResourceProvider",
		"DELETE",
		"/internal/provider",
		deregisterResourceProvider,
	},
	route{
		"getProviderResources",
		"GET",
		"/internal/resource/provider",
		getProviderResources,
	},
	route{
		"setResourceStatus",
		"POST",
		"/internal/resource/{resourceID}",
		setResourceStatus,
	},
	route{
		"getResource",
		"GET",
		"/internal/resource/{resourceID}",
		getResource,
	},
}

//
// Methods used by resource providers
//

func registerResourceProvider(w http.ResponseWriter, r *http.Request) {

	app := r.Context().Value("app").(*daemon.App)

	var provider daemon.Provider
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&provider)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = daemon.RegisterProvider(app, provider.Type)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deregisterResourceProvider(w http.ResponseWriter, r *http.Request) {

	app := r.Context().Value("app").(*daemon.App)

	var provider daemon.Provider
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&provider)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = daemon.DeregisterProvider(app, provider.Type)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func getProviderResources(w http.ResponseWriter, r *http.Request) {

	app := r.Context().Value("app").(*daemon.App)

	resources, err := daemon.GetProviderResources(app)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resources)
}

//
// Methods used by normal applications to manipulate their own resources
//

func getOwnResources(w http.ResponseWriter, r *http.Request) {

	app := r.Context().Value("app").(*daemon.App)
	resources := daemon.GetAppResources(app)

	json.NewEncoder(w).Encode(resources)

}

func createResource(w http.ResponseWriter, r *http.Request) {

	app := r.Context().Value("app").(*daemon.App)

	bodyJSON, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	resource, err := app.CreateResource(bodyJSON)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resource)

}

func getResource(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	resourceID := vars["resourceID"]
	log.Debug(resourceID)

	w.WriteHeader(http.StatusOK)

}

func deleteResource(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	resourceID := vars["resourceID"]

	app := r.Context().Value("app").(*daemon.App)
	err := app.DeleteResource(resourceID)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

}

func setResourceStatus(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	resourceID := vars["resourceID"]

	app := r.Context().Value("app").(*daemon.App)

	var status struct {
		Status string `json:"status"`
	}

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&status)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = app.SetResourceStatus(resourceID, status.Status)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

}
