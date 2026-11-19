# Include custom targets and environment variables here

.PHONY: deploy-auto
deploy-auto: ## Builds and deploys over the REST API, resolving redirects and logins itself.
	./build/deploy.sh
