package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	//"strconv"
	"bufio"
	"io"
)

type SubjectAccessReview struct {
	ApiVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Spec       SubjectSpec  `json:"spec,omitempty"`
	Status     AccessStatus `json:"status,omitempty"`
}

type SubjectSpec struct {
	ResourceAttributes SubjectAttributes `json:"resourceAttributes"`
	User               string            `json:"user"`
	Group              []string          `json:"group"`
}

type SubjectAttributes struct {
	Namespace string `json:"namespace"`
	Verb      string `json:"verb"`
	Group     string `json:"group"`
	Resource  string `json:"resource"`
}

type AccessStatus struct {
	Allowed bool   `json:"allowed"`
	Denied  bool   `json:"denied,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

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

	ret := SubjectAccessReview{
		ApiVersion: "authorization.k8s.io/v1beta1",
		Kind: "SubjectAccessReview",
		Status: AccessStatus{Allowed: true}
	}
	request := ""

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
		ret.Status.Allowed = false
		ret.Status.Reason = "Webhook: Not POST method"
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
	var req SubjectAccessReview
	b := []byte(request)
	err := json.Unmarshal(b, &req)
	if err != nil {
		ret.Status.Allowed = false
		ret.Status.Reason = "Webhook: JSON parse error."
		return
	}

	// do filterings
	if req.SubjectSpec.SubjectAttributes == nil { return }
	if !matchList(req.SubjectSpec.SubjectAttributes.Group, apigroups) {
		return
	}
	if !matchList(req.SubjectSpec.SubjectAttributes.Resource, resources) {
		return
	}
	if !matchList(req.SubjectSpec.SubjectAttributes.Verb, verbs) {
		return
	}
	ret.Status.Allowed = false
	ret.Status.Reason = "Webhook: denied"
	return
}

var (
	resource, apigroups, verbs []string
)

func main() {
	var resourcesFlag, apigroupsFlag, verbsFlag, port string
	flag.StringVar(&resourcesFlag, "resources", "", "Commma separated resource names to be denied")
	flag.StringVar(&apigroupsFlag, "apigroups", "", "Commma separated api group names to be denied")
	flag.StringVar(&verbsFlag, "verbs", "", "Commma separated verbs to be denied")
	flag.StringVar(&port, "port", "3000", "Listen port number")
	flag.Parse()

	resources = strings.Split(resourcesFlag, ",")
	apigroups = strings.Split(apigroupsFlag, ",")
	verbs = strings.Split(verbsFlag, ",")

	// route handler
	http.HandleFunc("/", apiRequest)

	// do serve
	err := http.ListenAndServe(":" + port, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
