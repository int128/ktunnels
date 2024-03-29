CLUSTER_NAME := ktunnels-e2e
KUBECONFIG := output/kubeconfig.yaml
export KUBECONFIG

all:

cluster: $(KUBECONFIG)
$(KUBECONFIG):
	kind create cluster --name $(CLUSTER_NAME)
	kubectl version

delete-cluster:
	kind delete cluster --name $(CLUSTER_NAME)
	-rm $(KUBECONFIG)

deploy:
	cd controller && kustomize edit set image controller="$(CONTROLLER_IMAGE)"
	kustomize build controller | kubectl apply -f -
	kubectl -n ktunnels-system rollout status deployment ktunnels-controller-manager
	kubectl -n ktunnels-system get deployment
	kustomize build ../config/samples | kubectl apply -f -
	kubectl get proxy
	kubectl get tunnel

test:
	kubectl rollout status deployment ktunnels-proxy-default
	kubectl get proxy
	kubectl get tunnel
	kubectl get deployment
	kubectl get service
	KTUNNELS_E2E_TEST=1 go test -v

logs-controller:
	-kubectl -n ktunnels-system logs -l control-plane=controller-manager --all-containers --tail=-1

logs-proxy:
	-kubectl logs -l ktunnels.int128.github.io/proxy=default --all-containers --tail=-1
