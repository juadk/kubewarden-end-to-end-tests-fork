ROOT_DIR:=$(realpath $(PWD)/..)

# Define Ginkgo timeout for the tests
GINKGO_TIMEOUT?=3600
ifdef VM_NUMBERS
	ifeq ($(shell expr $(VM_NUMBERS) \> 50), 1)
		GINKGO_TIMEOUT=7200
	endif
endif

deps: 
	@go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo
	@go install -mod=mod github.com/onsi/gomega
	@go mod tidy

# E2E tests
e2e-airgap-rancher: deps
	ginkgo --label-filter airgap-rancher -r -v ./e2e

e2e-configure-rancher: deps
	ginkgo --label-filter configure -r -v ./e2e

e2e-get-logs: deps
	ginkgo --label-filter logs -r -v ./e2e

e2e-install-chartmuseum:
	sudo ./scripts/deploy-chartmuseum $(OPERATOR_REPO)

e2e-install-rancher: deps
	ginkgo --label-filter install -r -v ./e2e

e2e-prepare-archive: deps
	ginkgo --label-filter prepare-archive -r -v ./e2e

e2e-ui-rancher: deps
	ginkgo --label-filter ui -r -v ./e2e

e2e-uninstall-operator:
	ginkgo --label-filter uninstall-operator -r -v ./e2e
