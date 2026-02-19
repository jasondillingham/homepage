#-----------------------------------------------------------------------------#
#--- Helpers
#-----------------------------------------------------------------------------#

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

#-----------------------------------------------------------------------------#
#--- Development
#-----------------------------------------------------------------------------#

## build: build the homepage binary
.PHONY: build
build:
	@echo "Building homepage..."
	go build -ldflags="-s" -o=./bin/homepage .

## run: build and run the application
.PHONY: run
run: build
	./bin/homepage

## dev: run with hot-reload on file changes (requires air)
.PHONY: dev
dev:
	go run github.com/air-verse/air@latest \
		-build.bin "./bin/homepage" \
		-build.cmd "make build" \
		-build.delay "1000" \
		-build.exclude_dir "bin,tmp,vendor,.git" \
		-build.include_ext "go,html,json,css,js" \
		-build.stop_on_error "true" \
		-build.send_interrupt "true" \
		-misc.clean_on_exit "true"

#-----------------------------------------------------------------------------#
#--- Quality Control
#-----------------------------------------------------------------------------#

## tidy: tidy modfiles and format .go files
.PHONY: tidy
tidy:
	@echo "Tidying modules and formatting code..."
	go mod tidy -v
	go fmt ./...

#-----------------------------------------------------------------------------#
#--- Staging (10.10.200.162)
#-----------------------------------------------------------------------------#

staging_host = '10.10.200.162'
staging_user = 'cms'
staging_dir  = '/opt/staging/homepage'

## staging-build: cross-compile for staging (linux/amd64)
.PHONY: staging-build
staging-build:
	@echo "Building for staging (linux/amd64)..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o=./bin/homepage .

## staging-connect: connect to the staging server
.PHONY: staging-connect
staging-connect:
	ssh ${staging_user}@${staging_host}

## staging-setup: create directories and install systemd service on staging
.PHONY: staging-setup
staging-setup:
	@echo "Setting up staging environment..."
	ssh ${staging_user}@${staging_host} 'sudo mkdir -p ${staging_dir}/templates/partials && sudo chown -R ${staging_user}:${staging_user} ${staging_dir}'
	scp homepage.service ${staging_user}@${staging_host}:/tmp/homepage.service
	ssh ${staging_user}@${staging_host} 'sudo mv /tmp/homepage.service /etc/systemd/system/homepage.service && sudo systemctl daemon-reload && sudo systemctl enable homepage'
	@echo "Staging environment ready."

## staging-deploy: build and deploy binary + assets to staging
.PHONY: staging-deploy
staging-deploy: staging-build
	@echo "Deploying to staging..."
	rsync -rP --delete ./bin/homepage ./templates ${staging_user}@${staging_host}:${staging_dir}/
	ssh ${staging_user}@${staging_host} 'sudo systemctl restart homepage'
	@echo ""
	@echo "Deploy complete:"
	@ssh ${staging_user}@${staging_host} 'sudo systemctl status homepage --no-pager'

## staging-logs: view staging logs (follows)
.PHONY: staging-logs
staging-logs:
	ssh -t ${staging_user}@${staging_host} 'sudo journalctl --unit=homepage --since="24 hours ago" --follow'

## staging-status: check staging service status
.PHONY: staging-status
staging-status:
	@ssh ${staging_user}@${staging_host} 'sudo systemctl status homepage --no-pager'

## staging-stop: stop staging service
.PHONY: staging-stop
staging-stop:
	ssh ${staging_user}@${staging_host} 'sudo systemctl stop homepage'
	@echo "homepage stopped"

## staging-start: start staging service
.PHONY: staging-start
staging-start:
	ssh ${staging_user}@${staging_host} 'sudo systemctl start homepage'
	@echo "homepage started"

## staging-restart: restart staging service
.PHONY: staging-restart
staging-restart:
	ssh ${staging_user}@${staging_host} 'sudo systemctl restart homepage'
	@echo "homepage restarted"

#-----------------------------------------------------------------------------#
#--- Git Workflow
#-----------------------------------------------------------------------------#

## git-status: show git status and changes
.PHONY: git-status
git-status:
	@echo "=== Git Status ==="
	@git status --short
	@echo "\n=== Recent Commits ==="
	@git log --oneline -5

## git-push: push to origin main (with safety check)
.PHONY: git-push
git-push:
	@echo "Pushing to origin/main..."
	@git pull --rebase origin main
	@git push origin main
