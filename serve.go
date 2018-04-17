package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

var (
	version = "Version 1.1"

	help = `
NAME
    static-file-server

SYNOPSIS
    static-file-server
    static-file-server [ help | -help | --help ]
    static-file-server [ version | -version | --version ]

DESCRIPTION
    The Static File Server is intended to be a tiny, fast and simple solution
    for serving files over HTTP. The features included are limited to make to
    binding to a host name and port, selecting a folder to serve, choosing a
    URL path prefix and selecting TLS certificates. If you want really awesome
    reverse proxy features, I recommend Nginx.

DEPENDENCIES
    None... not even libc!

ENVIRONMENT VARIABLES
    FOLDER
        The path to the folder containing the contents to be served over
        HTTP(s). If not supplied, defaults to '/web' (for Docker reasons).
    HOST
        The hostname used for binding. If not supplied, contents will be served
        to a client without regard for the hostname.
    PORT
        The port used for binding. If not supplied, defaults to port '8080'.
    SHOW_LISTING
        Automatically serve the index file for the directory if requested. For
        example, if the client requests 'http://127.0.0.1/' the 'index.html'
        file in the root of the directory being served is returned. If the value
        is set to 'false', the same request will return a 'NOT FOUND'. Default
        value is 'true'.
    TLS_CERT
        Path to the TLS certificate file to serve files using HTTPS. If supplied
        then TLS_KEY must also be supplied. If not supplied, contents will be
        served via HTTP.
    TLS_KEY
        Path to the TLS key file to serve files using HTTPS. If supplied then
        TLS_CERT must also be supplied. If not supplied, contents will be served
        via HTTPS
    URL_PREFIX
        The prefix to use in the URL path. If supplied, then the prefix must
        start with a forward-slash and NOT end with a forward-slash. If not
        supplied then no prefix is used.

USAGE
    FILE LAYOUT
       /var/www/sub/my.file
       /var/www/index.html

    COMMAND
        export FOLDER=/var/www/sub
        static-file-server
            Retrieve with: wget http://localhost:8080/my.file
                           wget http://my.machine:8080/my.file

        export FOLDER=/var/www
        export HOST=my.machine
        export PORT=80
        static-file-server
            Retrieve with: wget http://my.machine/sub/my.file

        export FOLDER=/var/www/sub
        export HOST=my.machine
        export PORT=80
        export URL_PREFIX=/my/stuff
        static-file-server
            Retrieve with: wget http://my.machine/my/stuff/my.file

        export FOLDER=/var/www/sub
        export TLS_CERT=/etc/server/my.machine.crt
        export TLS_KEY=/etc/server/my.machine.key
        static-file-server
            Retrieve with: wget https://my.machine:8080/my.file

        export FOLDER=/var/www/sub
        export PORT=443
        export TLS_CERT=/etc/server/my.machine.crt
        export TLS_KEY=/etc/server/my.machine.key
        static-file-server
            Retrieve with: wget https://my.machine/my.file

        export FOLDER=/var/www
        export PORT=80
        export SHOW_LISTING=true  # Default behavior
        static-file-server
            Retrieve 'index.html' with: wget http://my.machine/

        export FOLDER=/var/www
        export PORT=80
        export SHOW_LISTING=false
        static-file-server
            Returns 'NOT FOUND': wget http://my.machine/
`
)

func main() {
	// Evaluate and execute subcommand if supplied.
	if 1 < len(os.Args) {
		arg := os.Args[1]
		switch {
		case strings.Contains(arg, "help"):
			fmt.Println(help)
		case strings.Contains(arg, "version"):
			fmt.Println(version)
		default:
			name := os.Args[0]
			log.Fatalf("Unknown argument: %s. Try '%s help'.", arg, name)
		}
		return
	}

	// Collect environment variables.
	folder := env("FOLDER", "/web") + "/"
	host := env("HOST", "")
	port := env("PORT", "8080")
	showListing := envAsBool("SHOW_LISTING", true)
	tlsCert := env("TLS_CERT", "")
	tlsKey := env("TLS_KEY", "")
	urlPrefix := env("URL_PREFIX", "")

	// If HTTPS is to be used, verify both TLS_* environment variables are set.
	if 0 < len(tlsCert) || 0 < len(tlsKey) {
		if 0 == len(tlsCert) || 0 == len(tlsKey) {
			log.Fatalln(
				"If value for environment variable 'TLS_CERT' or 'TLS_KEY' is set " +
					"then value for environment variable 'TLS_KEY' or 'TLS_CERT' must " +
					"also be set.",
			)
		}
	}

	// If the URL path prefix is to be used, verify it is properly formatted.
	if 0 < len(urlPrefix) &&
		(!strings.HasPrefix(urlPrefix, "/") || strings.HasSuffix(urlPrefix, "/")) {
		log.Fatalln(
			"Value for environment variable 'URL_PREFIX' must start " +
				"with '/' and not end with '/'. Example: '/my/prefix'",
		)
	}

	// Choose and set the appropriate, optimized static file serving function.
	var handler http.HandlerFunc
	if 0 == len(urlPrefix) {
		handler = handleListing(showListing, basicHandler(folder))
	} else {
		handler = handleListing(showListing, prefixHandler(folder, urlPrefix))
	}
	http.HandleFunc("/", handler)

	// Serve files over HTTP or HTTPS based on paths to TLS files being provided.
	if 0 == len(tlsCert) {
		log.Fatalln(http.ListenAndServe(host+":"+port, nil))
	} else {
		log.Fatalln(http.ListenAndServeTLS(host+":"+port, tlsCert, tlsKey, nil))
	}
}

// handleListing wraps an HTTP request. In the event of a folder root request,
// setting 'show' to false will automatically return 'NOT FOUND' whereas true
// will attempt to retrieve the index file of that directory.
func handleListing(show bool, serve http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if show || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		serve(w, r)
	}
}

// basicHandler serves files from the folder passed.
func basicHandler(folder string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, folder+r.URL.Path)
	}
}

// prefixHandler removes the URL path prefix before serving files from the
// folder passed.
func prefixHandler(folder, urlPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, urlPrefix) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, folder+strings.TrimPrefix(r.URL.Path, urlPrefix))
	}
}

// env returns the value for an environment variable or, if not set, a fallback
// value.
func env(key, fallback string) string {
	if value := os.Getenv(key); 0 < len(value) {
		return value
	}
	return fallback
}

// strAsBool converts the intent of the passed value into a boolean
// representation.
func strAsBool(value string) (result bool, err error) {
	lvalue := strings.ToLower(value)
	switch lvalue {
	case "0", "false", "f", "no", "n":
		result = false
	case "1", "true", "t", "yes", "y":
		result = true
	default:
		result = false
		msg := "Unknown conversion from string to bool for value '%s'"
		err = fmt.Errorf(msg, value)
	}
	return
}

// envAsBool returns the value for an environment variable or, if not set, a
// fallback value as a boolean.
func envAsBool(key string, fallback bool) bool {
	value := env(key, fmt.Sprintf("%t", fallback))
	result, err := strAsBool(value)
	if nil != err {
		log.Printf(
			"Invalid value for '%s': %v\nUsing fallback: %t",
			key, err, fallback,
		)
		return fallback
	}
	return result
}
