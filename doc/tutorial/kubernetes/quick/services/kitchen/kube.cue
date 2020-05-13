package kube

#Component: "kitchen"

deployment: [string]: spec: template: {
	metadata: annotations: "prometheus.io.scrape": "true"
	spec: containers: [{
		ports: [{
			containerPort: 8080
		}]
		livenessProbe: {
			httpGet: {
				path: "/debug/health"
				port: 8080
			}
			initialDelaySeconds: 40
			periodSeconds:       3
		}
	}]
}

deployment: [ID=_]: spec: template: spec: {
	_hasDisks: *true | bool

	// field comprehension using just "if"
	if _hasDisks {
		volumes: [{
			name: *"\(ID)-disk" | string
			gcePersistentDisk: pdName: *"\(ID)-disk" | string
			gcePersistentDisk: fsType: "ext4"
		}, {
			name: *"secret-\(ID)" | string
			secret: secretName: *"\(ID)-secrets" | string
		}, ...]

		containers: [{
			volumeMounts: [{
				name:      *"\(ID)-disk" | string
				mountPath: *"/logs" | string
			}, {
				mountPath: *"/etc/certs" | string
				name:      *"secret-\(ID)" | string
				readOnly:  true
			}, ...]
		}]
	}
}
