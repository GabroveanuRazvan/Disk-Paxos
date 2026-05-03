.PHONY: run clear up down


run:
	go run cmd/main.go

# Clear the disks
clear:
	go run cmd/clear_disks/main.go

# Start the redis disks
up:
	docker compose up -d

# Stop the redis disks
down:
	docker compose down
