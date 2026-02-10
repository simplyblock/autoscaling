# Autoscaling Kustomize bundle

Unified install of NeonVM + autoscale components via a single `kubectl apply -k`.

## Apply
```sh
kubectl apply -k deploy/kustomize
```

## Contents
- NeonVM base (CRDs, webhooks, device plugin, RBAC, NADs)
- NeonVM controller, VXLAN controller, runner image-loader
- Autoscale scheduler + autoscaler agent
- Multus thick daemonset + Whereabouts

Images are pinned to the demo tags; change with `kustomize edit set image` or by editing `deploy/kustomize/kustomization.yaml`.
