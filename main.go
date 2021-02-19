package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
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

func errorResponse(code int32, reason string) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Code: code,
			Reason: metav1.StatusReason(reason),
		},
	}
}

func allowedResponse() *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

type admissionFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

func apiRequest(w http.ResponseWriter, r *http.Request, admFunc admissionFunc) {

	ret := v1beta1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1beta1",
		},
	}

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
		ret.Response = errorResponse(500, "Webhook: Not POST method")
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
		ret.Response = errorResponse(500, "Webhook: JSON parse error")
		return
	}
	if debug {
		klog.Infof(
			"Addmission webhook checking : %s/%s/%s\n",
			req.Request.Resource.Group,
			req.Request.Resource.Resource,
			req.Request.Operation,
		)
	}

	ret.Response = admFunc(req)
	ret.Response.UID = req.Request.UID
	return
}

var (
	debug bool
)

func mutatePspRequest(w http.ResponseWriter, r *http.Request) {
	apiRequest(w, r, mutatePodSecurityPolicies)
}

func main() {
	// flags
	var kubeconfig, port, certFile, keyFile string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&certFile, "tls-cert-file", "", "file path for TLS certificate")
	flag.StringVar(&keyFile, "tls-key-file", "", "file path for key of TLS certificate")
	flag.StringVar(&port, "port", "9443", "Listen port number")
	flag.BoolVar(&debug, "debug", false, "Print requested group/resource/operation")
	flag.Parse()

	// Config
	if kubeconfig != "" {
		go func() {
			config := newWebhookConfig(kubeconfig)
			config.Wait()
		}()
	}

	// route handler
	http.HandleFunc("/mutate-psp", mutatePspRequest)

	// do serve
	err := http.ListenAndServeTLS(":" + port, certFile, keyFile, nil)
	if err != nil {
		klog.Errorf("%s", err.Error())
		os.Exit(1)
	}
}
