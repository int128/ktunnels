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
	kubectl apply -f ../config/samples
	kubectl get proxy
	kubectl get tunnels

test:
	kubectl rollout status deployment ktunnels-proxy-default
	kubectl get proxy
	kubectl get tunnels

logs-controller:
	-kubectl -n ktunnels-system logs -l control-plane=controller-manager --all-containers --tail=-1
