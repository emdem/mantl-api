package api

import (
	"encoding/json"
	"fmt"
	"github.com/CiscoCloud/mantl-api/install"
	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	"io"
	"io/ioutil"
	"net/http"
)

type Api struct {
	listen  string
	install *install.Install
}

func NewApi(listen string, install *install.Install) *Api {
	return &Api{listen, install}
}

func (api *Api) Start() {
	router := httprouter.New()
	router.GET("/health", api.health)

	router.GET("/1/packages", api.packages)
	router.POST("/1/packages", api.installPackage)
	router.GET("/1/packages/:name", api.describePackage)
	router.DELETE("/1/packages/:name", api.uninstallPackage)

	log.WithField("port", api.listen).Info("Starting listener")
	log.Fatal(http.ListenAndServe(api.listen, router))
}

func (api *Api) health(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.WriteHeader(200)
	fmt.Fprintf(w, "OK")
}

func (api *Api) packages(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")
	packages, err := api.install.Packages()

	if err != nil {
		api.writeError(w, "Could not retrieve package list", err)
		return
	}

	if err = json.NewEncoder(w).Encode(packages); err != nil {
		api.writeError(w, "Could not retrieve package list", err)
	}
}

func (api *Api) describePackage(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	name := ps.ByName("name")
	pkg, err := api.install.Package(name)
	if err != nil {
		api.writeError(w, fmt.Sprintf("Could not find package %s", name), err)
		return
	}

	if err = json.NewEncoder(w).Encode(pkg); err != nil {
		api.writeError(w, fmt.Sprintf("Could not encode package %s", name), err)
	}
}

func (api *Api) installPackage(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	req.Header.Add("Accept", "application/json")

	pkgRequest, err := api.parsePackageRequest(req.Body, "")

	if err != nil || pkgRequest == nil {
		api.writeError(w, "Could not parse package request", err)
		return
	}

	marathonResponse, err := api.install.InstallPackage(pkgRequest)
	if err != nil {
		api.writeError(w, fmt.Sprintf("Could not install %s package", pkgRequest.Name), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, marathonResponse)
}

func (api *Api) uninstallPackage(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	req.Header.Add("Accept", "application/json")

	name := ps.ByName("name")
	pkgRequest, err := api.parsePackageRequest(req.Body, name)

	if err != nil || pkgRequest == nil {
		api.writeError(w, "Could not parse package request", err)
		return
	}

	app := api.install.FindInstalled(pkgRequest)
	if app == nil {
		w.WriteHeader(404)
		return
	}

	err = api.install.UninstallPackage(app)
	if err != nil {
		api.writeError(w, fmt.Sprintf("Could not uninstall %s package", pkgRequest.Name), err)
		return
	}

	w.WriteHeader(204)
}

func (api *Api) writeError(w http.ResponseWriter, msg string, err error) {
	w.WriteHeader(500)
	m := fmt.Sprintf("%s: %v", msg, err)
	log.Error(m)
	fmt.Fprintln(w, m)
}

func (api *Api) parsePackageRequest(r io.Reader, pkgName string) (*install.PackageRequest, error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		log.Errorf("Unable to read request body: %v", err)
		return nil, err
	}

	pkgRequest, err := install.NewPackageRequest(body)
	if err != nil {
		return nil, err
	}

	if pkgName != "" {
		pkgRequest.Name = pkgName
	}

	return pkgRequest, nil
}
