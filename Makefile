SHELL := /bin/bash

FRONTEND_DEV_CMD = cd frontend && npm run dev
BACKEND_DEV_CMD = cd backend && go run cmd/server/main.go

.PHONY: dev dev-frontend dev-backend

dev:
	@backend_pid=; \
	frontend_pid=; \
	status=0; \
	cleanup() { \
		[ -z "$$backend_pid" ] || kill "$$backend_pid" 2>/dev/null || true; \
		[ -z "$$frontend_pid" ] || kill "$$frontend_pid" 2>/dev/null || true; \
	}; \
	handle_interrupt() { \
		status=130; \
		cleanup; \
	}; \
	trap cleanup EXIT INT TERM; \
	trap handle_interrupt INT TERM; \
	($(BACKEND_DEV_CMD)) & \
	backend_pid=$$!; \
	($(FRONTEND_DEV_CMD)) & \
	frontend_pid=$$!; \
	while true; do \
		if [ "$$status" -ne 0 ]; then \
			break; \
		fi; \
		if ! kill -0 "$$backend_pid" 2>/dev/null; then \
			wait "$$backend_pid"; \
			status=$$?; \
			break; \
		fi; \
		if ! kill -0 "$$frontend_pid" 2>/dev/null; then \
			wait "$$frontend_pid"; \
			status=$$?; \
			break; \
		fi; \
		sleep 1; \
	done; \
	cleanup; \
	wait "$$backend_pid" 2>/dev/null || true; \
	wait "$$frontend_pid" 2>/dev/null || true; \
	if [ "$$status" -eq 130 ]; then \
		exit 0; \
	fi; \
	exit $$status

dev-frontend:
	@$(FRONTEND_DEV_CMD)

dev-backend:
	@$(BACKEND_DEV_CMD)
