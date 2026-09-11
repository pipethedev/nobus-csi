# Nobus CSI Helm Chart

Install the Nobus CSI driver on Kubernetes 1.28+.

```sh
helm install nobus-csi ./deploy/helm/nobus-csi \
  --namespace kube-system \
  --set nobus.projectId="$NOBUS_PROJECT_ID" \
  --set nobus.availabilityZone="$NOBUS_AVAILABILITY_ZONE" \
  --set nobus.apiKey="$NOBUS_API_KEY"
```

Use an existing Secret instead of putting credentials in Helm values:

```sh
kubectl -n kube-system create secret generic nobus-csi \
  --from-literal=NOBUS_API_URL=https://cloud-api.nobus.io \
  --from-literal=NOBUS_API_KEY="$NOBUS_API_KEY" \
  --from-literal=NOBUS_PROJECT_ID="$NOBUS_PROJECT_ID" \
  --from-literal=NOBUS_AVAILABILITY_ZONE="$NOBUS_AVAILABILITY_ZONE"

helm install nobus-csi ./deploy/helm/nobus-csi \
  --namespace kube-system \
  --set nobus.existingSecret=nobus-csi \
  --set nobus.projectId="$NOBUS_PROJECT_ID" \
  --set nobus.availabilityZone="$NOBUS_AVAILABILITY_ZONE"
```
