package api

import (
	"context"
	"crypto/tls"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/protosio/protos/meta"
	"github.com/protosio/protos/resource"

	// statik package is use to embed static web assets in the protos binary
	_ "github.com/protosio/protos/statik"

	"github.com/protosio/protos/capability"
	"github.com/protosio/protos/config"
	"github.com/protosio/protos/util"

	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/unrolled/render"
	"github.com/urfave/negroni"
)

var log = util.GetLogger("api")
var gconfig = config.Get()
var rend = render.New(render.Options{IndentJSON: true})

type httperr struct {
	Error string `json:"error"`
}

type route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
	Capability  *capability.Capability
}

type routes []route

func uiHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, string(http.Dir(gconfig.StaticAssets))+"/index.html")
}

func uiRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/ui/", 303)
}

func applyAPIroutes(r *mux.Router, routes []route) *mux.Router {
	for _, route := range routes {
		if route.Method != "" {
			// if route method is set (GET, POST etc), the route is only valid for that method
			r.Methods(route.Method).Path(route.Pattern).Name(route.Name).Handler(route.HandlerFunc)
		} else {
			// if route method is not set, it will work for all methods. Useful for WS
			r.Path(route.Pattern).Name(route.Name).Handler(route.HandlerFunc)
		}
		if route.Capability != nil {
			capability.SetMethodCap(route.Name, route.Capability)
		}
	}
	return r
}

func applyAuthRoutes(r *mux.Router, enableRegister bool) {
	// Authentication routes
	authRouter := mux.NewRouter().PathPrefix("/api/v1/auth").Subrouter().StrictSlash(true)
	if enableRegister == true {
		authRouter.Methods("POST").Path("/register").Name("register").Handler(http.HandlerFunc(RegisterHandler))
	}
	authRouter.Methods("POST").Path("/login").Name("login").Handler(http.HandlerFunc(LoginHandler))

	r.PathPrefix("/api/v1/auth").Handler(authRouter)
}

func applyInternalAPIroutes(r *mux.Router) *mux.Router {

	// Internal routes
	internalRouter := mux.NewRouter().PathPrefix("/api/v1/i").Subrouter().StrictSlash(true)
	for _, route := range internalRoutes {
		internalRouter.Methods(route.Method).Path(route.Pattern).Name(route.Name).Handler(route.HandlerFunc)
		if route.Capability != nil {
			capability.SetMethodCap(route.Name, route.Capability)
		}
	}

	r.PathPrefix("/api/v1/i").Handler(negroni.New(
		negroni.HandlerFunc(InternalRequestValidator),
		negroni.Wrap(internalRouter),
	))
	return internalRouter
}

func applyExternalAPIroutes(r *mux.Router) *mux.Router {

	// External routes (require auth)
	externalRouter := mux.NewRouter().PathPrefix("/api/v1/e").Subrouter().StrictSlash(true)
	for _, route := range externalRoutes {
		if route.Method != "" {
			externalRouter.Methods(route.Method).Path(route.Pattern).Name(route.Name).Handler(route.HandlerFunc)
		} else {
			externalRouter.Path(route.Pattern).Name(route.Name).Handler(route.HandlerFunc)
		}
		if route.Capability != nil {
			capability.SetMethodCap(route.Name, route.Capability)
		}
	}

	r.PathPrefix("/api/v1/e").Handler(negroni.New(
		negroni.HandlerFunc(ExternalRequestValidator),
		negroni.Wrap(externalRouter),
	))
	return externalRouter
}

func applyStaticRoutes(r *mux.Router) {
	// UI routes
	var fileHandler http.Handler
	if gconfig.StaticAssets != "" {
		log.Debugf("Running webserver with static assets from %s", gconfig.StaticAssets)
		fileHandler = http.FileServer(http.Dir(gconfig.StaticAssets))
		r.PathPrefix("/ui/").Name("ui").Handler(http.HandlerFunc(uiHandler))
	} else {
		statikFS, err := fs.New()
		if err != nil {
			log.Fatal(err)
		}
		log.Debug("Running webserver with embedded static assets")
		fileHandler = http.FileServer(statikFS)
		file, err := statikFS.Open("/index.html")
		if err != nil {
			log.Fatal(errors.Wrap(err, "Failed to open the embedded index.html file"))
		}
		r.PathPrefix("/ui/").Name("ui").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "index.html", time.Now(), file)
		}))
	}
	r.PathPrefix("/static/").Name("static").Handler(http.StripPrefix("/static/", fileHandler))
	r.PathPrefix("/").Name("root").Handler(http.HandlerFunc(uiRedirect))

}

func secureListen(handler http.Handler, certrsc resource.Type, quit chan bool) {
	cert, ok := certrsc.(*resource.CertificateResource)
	if ok == false {
		log.Fatal("Failed to read TLS certificate")
	}
	tlscert, err := tls.X509KeyPair(cert.Certificate, cert.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to parse the TLS certificate: %s", err.Error())
	}
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		Certificates: []tls.Certificate{tlscert},
	}

	httpsport := strconv.Itoa(gconfig.HTTPSport)
	httpport := strconv.Itoa(gconfig.HTTPport)
	srv := &http.Server{
		Addr:         ":" + httpsport,
		Handler:      handler,
		TLSConfig:    cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}

	// holds all the internal web servers
	internalSrvs := []*http.Server{}

	var ips []string
	if gconfig.InternalIP != "" {
		ips = []string{gconfig.InternalIP}
	} else {
		ips, err = util.GetLocalIPs()
		if err != nil {
			log.Fatal(errors.Wrap(err, "Failed to start HTTPS server"))
		}
	}
	for _, nip := range ips {
		ip := nip
		log.Infof("Listening internally on %s:%s (HTTP)", ip, httpport)
		isrv := &http.Server{Addr: ip + ":" + httpport, Handler: handler}
		internalSrvs = append(internalSrvs, isrv)
		go func() {
			if err := isrv.ListenAndServe(); err != nil {
				if strings.Contains(err.Error(), "Server closed") {
					log.Infof("Internal (%s) API webserver terminated successfully", ip)
				} else {
					log.Errorf("Internal (%s) API webserver died with error: %s", ip, err.Error())
				}
			}
		}()
	}

	go func() {
		log.Infof("Listening on %s (HTTPS)", srv.Addr)
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			if strings.Contains(err.Error(), "Server closed") {
				log.Info("HTTPS API webserver terminated successfully")
			} else {
				log.Errorf("HTTPS API webserver died with error: %s", err.Error())
			}
		}
	}()

	<-quit
	log.Info("Shutting down HTTPS webserver")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error(errors.Wrap(err, "Something went wrong while shutting down the HTTPS webserver"))
	}

	for _, isrv := range internalSrvs {
		if err := isrv.Shutdown(context.Background()); err != nil {
			log.Error(errors.Wrap(err, "Something went wrong while shutting down the internal API webserver"))
		}
	}
}

// Websrv starts an HTTP(S) server that exposes all the application functionality
func Websrv(quit chan bool) {

	mainRtr := mux.NewRouter().StrictSlash(true)
	applyAuthRoutes(mainRtr, false)
	internalRouter := applyInternalAPIroutes(mainRtr)
	applyAPIroutes(internalRouter, internalWSRoutes)
	externalRouter := applyExternalAPIroutes(mainRtr)
	applyAPIroutes(externalRouter, externalWSRoutes)
	applyStaticRoutes(mainRtr)

	// Negroni middleware
	n := negroni.New()
	n.Use(negroni.HandlerFunc(HTTPLogger))
	n.UseHandler(mainRtr)

	cert := meta.GetTLSCertificate()
	secureListen(n, cert.Value, quit)
}

// WebsrvInit starts an HTTP server used only during the initialisation process
func WebsrvInit(quit chan bool) bool {
	mainRtr := mux.NewRouter().StrictSlash(true)
	applyAuthRoutes(mainRtr, true)
	internalRouter := applyInternalAPIroutes(mainRtr)
	applyAPIroutes(internalRouter, internalWSRoutes)
	externalRouter := applyExternalAPIroutes(mainRtr)
	applyAPIroutes(externalRouter, externalInitRoutes)
	applyAPIroutes(externalRouter, externalWSRoutes)
	applyStaticRoutes(mainRtr)

	// Negroni middleware
	n := negroni.New()
	n.Use(negroni.HandlerFunc(HTTPLogger))
	n.UseHandler(mainRtr)

	httpport := strconv.Itoa(gconfig.HTTPport)
	srv := &http.Server{
		Addr:           "0.0.0.0:" + httpport,
		Handler:        n,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Info("Starting init webserver on " + srv.Addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if strings.Contains(err.Error(), "Server closed") {
				log.Info("Init webserver terminated successfully")
			} else {
				log.Errorf("Init webserver died with error: %s", err.Error())
			}
		}
	}()

	interrupted := <-quit
	log.Info("Shutting down init webserver")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error(errors.Wrap(err, "Something went wrong while shutting down the init webserver"))
	}
	return interrupted
}
