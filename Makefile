.PHONY: help full infra-up infra-down db-import run-gateway run-user run-booking run-chat proto proto-user proto-booking test

POWERSHELL := powershell -NoProfile -ExecutionPolicy Bypass -Command

help:
	@echo "Available targets:"
	@echo "  make full         - start Docker infra and open all Go services in separate PowerShell windows"
	@echo "  make infra-up     - start postgres, redis and kafka"
	@echo "  make infra-down   - stop Docker infra"
	@echo "  make db-import    - import dump.sql into postgres"
	@echo "  make run-user     - run usermanagement-service"
	@echo "  make run-booking  - run booking-service"
	@echo "  make run-chat     - run chat-service"
	@echo "  make run-gateway  - run api-gateway"
	@echo "  make proto        - regenerate all protobuf Go files"
	@echo "  make test         - run go test ./..."

full: infra-up
	$(POWERSHELL) "Start-Process powershell -ArgumentList '-NoExit','-Command','cd \"$(CURDIR)\"; go run ./usermanagement-service/cmd'"
	$(POWERSHELL) "Start-Process powershell -ArgumentList '-NoExit','-Command','cd \"$(CURDIR)\"; go run ./booking-service/cmd'"
	$(POWERSHELL) "Start-Process powershell -ArgumentList '-NoExit','-Command','cd \"$(CURDIR)\"; go run ./chat-service/cmd'"
	$(POWERSHELL) "Start-Process powershell -ArgumentList '-NoExit','-Command','cd \"$(CURDIR)\"; go run ./api-gateway/cmd'"

infra-up:
	docker compose up -d

infra-down:
	docker compose down

db-import:
	docker exec -i postgres psql -U user -d app < dump.sql

run-user:
	go run ./usermanagement-service/cmd

run-booking:
	go run ./booking-service/cmd

run-chat:
	go run ./chat-service/cmd

run-gateway:
	go run ./api-gateway/cmd

proto: proto-user proto-booking

proto-user:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative api/usermanagement-service-proto/usermanagement.proto

proto-booking:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative api/booking-service-proto/booking.proto

test:
	$(POWERSHELL) "$$env:GOCACHE = Join-Path (Get-Location) '.gocache'; go test ./..."
