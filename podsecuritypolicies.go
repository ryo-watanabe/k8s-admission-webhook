package main

import (
	"encoding/json"

	"k8s.io/api/admission/v1beta1"
	policy "k8s.io/api/policy/v1beta1"
	"k8s.io/klog"
)

var (
	psp_apigroups = []string{"policy"}
	psp_resource string = "podsecuritypolicies"
	psp_mutate_operations = []string{"CREATE","UPDATE"}
)

const (
	removePrivileged string = `{"op":"remove","path":"/spec/privileged"}`
	removeHostPID string = `{"op":"remove","path":"/spec/hostPID"}`
	removeHostIPC string = `{"op":"remove","path":"/spec/hostIPC"}`
	removeHostNetwork string = `{"op":"remove","path":"/spec/hostNetwork"}`
	removeHostPorts string = `{"op":"remove","path":"/spec/hostPorts"}`
	removeSysctls string = `{"op":"remove","path":"/spec/allowedUnsafeSysctls"}`
	removeCapabilities string = `{"op":"remove","path":"/spec/allowedCapabilities"}`
	exceptHostpath string = `{"op":"replace","path":"/spec/volumes","value":["configMap","downwardAPI","emptyDir","persistentVolumeClaim","secret","projected"]}`
	replaceEtcHostsOnly string = `{"op":"replace","path":"/spec/allowedHostPaths","value":[{"pathPrefix":"/etc/hosts"}]}`
	replaceHostPort string = `{"op":"replace","path":"/spec/hostPorts","value":[{"max":65535,"min":20000}]}`

	hostPortMin int32 = 20000
	hostPortMax int32 = 65535
)

func hostpathInVolumes(volumes []policy.FSType) bool {
	for _, v := range(volumes) {
		if v == "hostPath" || v == "*" {
			return true
		}
	}
	return false
}

var allowedPaths = []policy.AllowedHostPath{
	policy.AllowedHostPath{ PathPrefix: "/etc/hosts" },
	policy.AllowedHostPath{ PathPrefix: "/lib/modules", ReadOnly: true },
	policy.AllowedHostPath{ PathPrefix: "/var/run/calico" },
	policy.AllowedHostPath{ PathPrefix: "/var/lib/calico" },
	policy.AllowedHostPath{ PathPrefix: "/run/xtables.lock" },
	policy.AllowedHostPath{ PathPrefix: "/sys/fs/" },
	policy.AllowedHostPath{ PathPrefix: "/opt/cni/bin" },
	policy.AllowedHostPath{ PathPrefix: "/etc/cni/net.d" },
	policy.AllowedHostPath{ PathPrefix: "/var/log/calico/cni", ReadOnly: true },
	policy.AllowedHostPath{ PathPrefix: "/var/run/nodeagent" },
	policy.AllowedHostPath{ PathPrefix: "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/nodeagent~uds" },
}

func pathInAllowedPaths(path policy.AllowedHostPath) bool {
	for _, p := range(allowedPaths) {
		if path.PathPrefix == p.PathPrefix && path.ReadOnly == p.ReadOnly {
			return true
		}
	}
	return false
}

func containsNotAllowedPath(paths []policy.AllowedHostPath) bool {
	for _, p := range(paths) {
		if !pathInAllowedPaths(p) {
			return true
		}
	}
	return false
}

func patchItemAdd(patch, item string) string {
	if item == "" {
		return patch + "]"
	}
	if patch != "" {
		return patch + "," + item
	}
	return "[" + item
}

func mutatePodSecurityPolicies(req v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {

	// check resource and operations
	if !matchList(req.Request.Resource.Group, psp_apigroups) {
		klog.Warningf("API group %s not matched for mutatePodSecurityPolicies - request allowed", req.Request.Resource.Group)
		return allowedResponse()
	}
	if req.Request.Resource.Resource != psp_resource {
		klog.Warningf("Resouce %s not matched for mutatePodSecurityPolicies - request allowed", req.Request.Resource.Resource)
		return allowedResponse()
	}
	if !matchList(string(req.Request.Operation), psp_mutate_operations) {
		klog.Warningf("Operation %s not matched for mutatePodSecurityPolicies - request allowed", req.Request.Operation)
		return allowedResponse()
	}

	// make patch
	klog.Infof("Patching : %s", string(req.Request.Object.Raw))
	psp := &policy.PodSecurityPolicy{}
	err := json.Unmarshal(req.Request.Object.Raw, &psp)
	if err != nil {
		return errorResponse(500, "Webhook: JSON parse object error")
	}
	patch := ""
	if psp.Spec.Privileged {
		patch = patchItemAdd(patch, removePrivileged)
	}
	if psp.Spec.HostPID {
		patch = patchItemAdd(patch, removeHostPID)
	}
	if psp.Spec.HostIPC {
		patch = patchItemAdd(patch, removeHostIPC)
	}
	//if psp.Spec.HostNetwork {
	//	patch = patchItemAdd(patch, removeHostNetwork)
	//}
	if len(psp.Spec.HostPorts) > 0 {
		rangeMutation := false
		restricted := false
		for _, r := range(psp.Spec.HostPorts) {
			if r.Min < hostPortMin || r.Max > hostPortMax {
				restricted = true
			}
			if r.Min <= hostPortMin && r.Max >= hostPortMax {
				patch = patchItemAdd(patch, replaceHostPort)
				rangeMutation = true
				break
			}
		}
		if restricted && !rangeMutation {
			patch = patchItemAdd(patch, removeHostPorts)
		}
	}
	if len(psp.Spec.AllowedCapabilities) > 0 {
		patch = patchItemAdd(patch, removeCapabilities)
	}
	if len(psp.Spec.AllowedUnsafeSysctls) > 0 {
		patch = patchItemAdd(patch, removeSysctls)
	}
	if len(psp.Spec.Volumes) > 0 && hostpathInVolumes(psp.Spec.Volumes) {
		if len(psp.Spec.AllowedHostPaths) > 0 {
			if containsNotAllowedPath(psp.Spec.AllowedHostPaths) {
				patch = patchItemAdd(patch, replaceEtcHostsOnly)
			}
		} else {
			patch = patchItemAdd(patch, replaceEtcHostsOnly)
		}
	}
	patch = patchItemAdd(patch, "")
	pt := v1beta1.PatchTypeJSONPatch

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch: []byte(patch),
		PatchType: &pt,
	}
}

