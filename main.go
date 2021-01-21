package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"bufio"
	"io"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func matchList(item string, list []string) bool {
	if item == "" {
		return false
	}
	matched := false
	for _, i := range(list) {
		if i == item || i == "*" {
			matched = true
		}
	}
	return matched
}

func apiRequest(w http.ResponseWriter, r *http.Request) {

	ret := v1beta1.AdmissionReview{}
	ret.Response.Allowed = true

	// JSON return
	defer func() {
		// result
		outjson, err := json.Marshal(ret)
		if err != nil {
			fmt.Println(err) //TODO: change to log
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(outjson))
	}()

	// type check
	if r.Method != "POST" {
		ret.Response.Allowed = false
		ret.Response.Result = &metav1.Status{
			Code: "500",
			Reason: "Webhook: Not POST method",
		}
		return
	}

	// request body
	rb := bufio.NewReader(r.Body)
	for {
		s, err := rb.ReadString('\n')
		request = request + s
		if err == io.EOF {
			break
		}
	}

	// JSON parse
	req := v1beta1.AdmissionReview{}
	b := []byte(request)
	err := json.Unmarshal(b, &req)
	if err != nil {
		ret.Response.Allowed = false
		ret.Response.Result = &metav1.Status{
			Code: "500",
			Reason: "Webhook: JSON parse error",
		}
		return
	}
	ret.Response.UID = req.Request.UID

	// do filterings
	if req.Request.Resource == nil {
		return
	}
	if debug {
		fmt.Printf(
			"Addmission webhook checking : %s/%s/%s\n",
			req.Request.Resource.Group,
			req.Request.Resource.Resource,
			req.Request.Resource.Operation,
		)
	}
	if !matchList(req.Request.Resource.Group, apigroups) {
		return
	}
	if !matchList(req.Request.Resource.Resource, resources) {
		return
	}
	if !matchList(req.Request.Operation, operations) {
		return
	}
	ret.Response.Allowed = false
	ret.Response.Result = &metav1.Status{
		Code: "403",
		Reason: "Webhook: denied",
	}
	return
}

var (
	resources, apigroups, operations []string
	debug bool
)

func main() {
	var resourcesFlag, apigroupsFlag, operationsFlag, port, certFile, keyFile string
	flag.StringVar(&resourcesFlag, "resources", "", "Commma separated resource names to be denied")
	flag.StringVar(&apigroupsFlag, "apigroups", "", "Commma separated api group names to be denied")
	flag.StringVar(&operationsFlag, "operations", "", "Commma separated operations to be denied")
	flag.StringVar(&certFile, "tls-cert-file", "", "file path for TLS certificate")
	flag.StringVar(&keyFile, "tls-key-file", "", "file path for key of TLS certificate")
	flag.StringVar(&port, "port", "9443", "Listen port number")
	flag.BoolVar(&debug, "debug", false, "Print requested group/resource/operation")
	flag.Parse()

	resources = strings.Split(resourcesFlag, ",")
	apigroups = strings.Split(apigroupsFlag, ",")
	operations = strings.Split(operationsFlag, ",")

	// route handler
	http.HandleFunc("/", apiRequest)

	// do serve
	err := http.ListenAndServeTLS(":" + port, certFile, keyFile, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
