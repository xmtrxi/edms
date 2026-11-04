DOCLING_COMPOSE    := docker compose -p edms-docling -f docker-compose.docling.yml
ONLYOFFICE_COMPOSE := docker compose -p edms-onlyoffice -f docker-compose.onlyoffice.yml

.PHONY: help docling-start docling-stop docling-restart docling-logs docling-status \
        onlyoffice-start onlyoffice-stop onlyoffice-logs onlyoffice-status

help:
	@echo "Docling:    make docling-start | docling-stop | docling-restart | docling-logs | docling-status"
	@echo "ONLYOFFICE: make onlyoffice-start | onlyoffice-stop | onlyoffice-logs | onlyoffice-status"

# --- Docling (docling-serve API + RQ worker + Gotenberg) ---
# Requires Redis on 127.0.0.1:6379 (services use host networking).

.env.docling:
	@touch .env.docling
	@echo "Created empty .env.docling (add DOCLING_SERVE_* overrides here if needed)"

docling-start: .env.docling
	$(DOCLING_COMPOSE) up -d
	@echo "Docling API: http://localhost:5001  (UI: http://localhost:5001/ui)"
	@echo "Gotenberg:   http://localhost:6000"

docling-stop:
	$(DOCLING_COMPOSE) down

docling-restart: docling-stop docling-start

docling-logs:
	$(DOCLING_COMPOSE) logs -f --tail=100

docling-status:
	$(DOCLING_COMPOSE) ps

# --- ONLYOFFICE ---

onlyoffice-start:
	$(ONLYOFFICE_COMPOSE) up -d

onlyoffice-stop:
	$(ONLYOFFICE_COMPOSE) down

onlyoffice-logs:
	$(ONLYOFFICE_COMPOSE) logs -f --tail=100

onlyoffice-status:
	$(ONLYOFFICE_COMPOSE) ps
