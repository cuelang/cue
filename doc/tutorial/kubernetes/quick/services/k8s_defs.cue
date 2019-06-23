package kube

import (
  "k8s.io/api/core/v1"
  extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
  appsv1 "k8s.io/api/apps/v1"
)

service <Name>: v1.Service & {}
deployment <Name>: extensions_v1beta1.Deployment & {}
daemonSet <Name>: extensions_v1beta1.DaemonSet & {}
statefulSet <Name>: appsv1.StatefulSet & {}
