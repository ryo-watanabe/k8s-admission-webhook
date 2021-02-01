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
	"k8s.io/klog"
)

func matchList(item string, list []string) bool {
	matched := false
	for _, i := range(list) {
		if i == item || i == "*" {
			matched = true
		}
	}
	return matched
}

func setErrorResponse(r *v1beta1.AdmissionResponse, code int32, reason string) {
	r.Allowed = false
	r.Result = &metav1.Status{
		Code: code,
		Reason: metav1.StatusReason(reason),
	}
}

func apiRequest(w http.ResponseWriter, r *http.Request) {

	ret := v1beta1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1beta1",
		}}
	ret.Response = &v1beta1.AdmissionResponse{}
	ret.Response.Allowed = true

	// JSON return
	defer func() {
		outjson, err := json.Marshal(ret)
		if err != nil {
			klog.Errorf("%s", err.Error())
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(outjson))
	}()

	// type check
	if r.Method != "POST" {
		setErrorResponse(ret.Response, 500, "Webhook: Not POST method")
		return
	}

	// request body
	rb := bufio.NewReader(r.Body)
	request := ""
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
		setErrorResponse(ret.Response, 500, "Webhook: JSON parse error")
		return
	}
	ret.Response.UID = req.Request.UID
	if debug {
		klog.Infof(
			"Addmission webhook checking : %s/%s/%s\n",
			req.Request.Resource.Group,
			req.Request.Resource.Resource,
			req.Request.Operation,
		)
	}

	// do filterings
	if !matchList(req.Request.Resource.Group, apigroups) {
		return
	}
	if !matchList(req.Request.Resource.Resource, resources) {
		return
	}
	if !matchList(string(req.Request.Operation), operations) {
		return
	}
	setErrorResponse(ret.Response, 403, "Webhook: denied")
	return
}

var (
	resources, apigroups, operations []string
	debug bool
)

func main() {
	// flags
	var kubeconfig, resourcesFlag, apigroupsFlag, operationsFlag, port, certFile, keyFile string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
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

	// Config
	if kubeconfig != "" {
		go func() {
			config := newWebhookConfig(kubeconfig)
			config.Wait()
		}()
	}

	// route handler
	http.HandleFunc("/", apiRequest)

	// do serve
	err := http.ListenAndServeTLS(":" + port, certFile, keyFile, nil)
	if err != nil {
		klog.Errorf("%s", err.Error())
		os.Exit(1)
	}
}
