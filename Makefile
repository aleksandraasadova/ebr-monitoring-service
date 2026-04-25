include .env 
export

export PROJECT_ROOT=$(shell pwd)

env-up:
	docker compose up -d ebr-postgres

env-down:
	docker compose down

env-clean:
	docker compose down
	sudo rm -rf ./out/pgdata

migrate-create:
	@if [ -z "$(seq)" ]; then \
		echo "Empty seq parameter. Example: make migrate-create seq=init"; \
		exit 1; \
	fi;
	docker-compose run --rm ebr-postgres-migrate \
		create \
		-ext sql \
		-dir /migrations \
		-seq "$(seq)"

migrate-up:
	make migrate-action action=up

migrate-down: 
	make migrate-action action=down

migrate-action:
	@if [ -z "$(action)" ]; then \
		echo "Empty action parameter. Example: make migrate-create action="down 1""; \
		exit 1; \
	fi;
	docker-compose run --rm ebr-postgres-migrate \
		-path /migrations \
		-database postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@ebr-postgres:5432/$(POSTGRES_DB)?sslmode=disable \
		"$(action)"

 ebr-run:
	@go run cmd/ebr/main.go