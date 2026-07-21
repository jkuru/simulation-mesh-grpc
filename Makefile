# Monorepo root — platform golden path first (docs/GOLDEN_PATH.md)
APP       := apps/reference-app
FRAMEWORK := apps/virtualization-framework
EXAMPLE   := examples/reference-app-with-framework

.PHONY: help demo unit coverage test up down generate clean deploy-dev mesh-e2e mesh-test \
	status framework-status example-status framework-test framework-coverage framework-install \
	framework-image framework-golden example-e2e platform-accept platform-e2e

help:
	@echo "servicemesh monorepo — platform engineering track"
	@echo ""
	@echo "  GOLDEN PATH (supported product surface)"
	@echo "    make platform-accept   offline: coverage + goldens + example guard"
	@echo "    make platform-e2e      cluster: framework + CR + app proof"
	@echo "    docs: docs/GOLDEN_PATH.md  docs/PLATFORM.md"
	@echo ""
	@echo "  Learning / internals"
	@echo "    make demo              local app (no mesh)"
	@echo "    make mesh-e2e          hand-written Istio teaching path"
	@echo "    make coverage          reference-app internal/ 100%"
	@echo "    make framework-coverage"
	@echo "    make framework-golden"
	@echo ""
	@echo "  Paths: $(APP) | $(FRAMEWORK) | $(EXAMPLE)"

# --- Platform acceptance ----------------------------------------------------

platform-accept:
	@chmod +x ./scripts/platform-accept.sh
	@./scripts/platform-accept.sh

# Cluster golden path = consumer example e2e (framework generates Istio).
platform-e2e example-e2e:
	@$(MAKE) -C $(EXAMPLE) e2e

# --- App / framework shortcuts ----------------------------------------------

demo unit coverage test up down generate clean deploy-dev mesh-e2e mesh-test:
	@$(MAKE) -C $(APP) $@

framework-test:
	@$(MAKE) -C $(FRAMEWORK) test

framework-coverage:
	@$(MAKE) -C $(FRAMEWORK) coverage

framework-golden:
	@$(MAKE) -C $(FRAMEWORK) golden

framework-image:
	@$(MAKE) -C $(FRAMEWORK) image

framework-install:
	@$(MAKE) -C $(FRAMEWORK) install

status: framework-status example-status
	@echo ""
	@echo "==> $(APP)"
	@grep -E '^\| \*\*State\*\*|^# STATUS|^\| \*\*App|^\| \*\*Phase|^\| \*\*v3|^\| \*\*v1' $(APP)/STATUS.md | head -10 || true

framework-status:
	@echo "==> $(FRAMEWORK)"
	@sed -n '1,25p' $(FRAMEWORK)/STATUS.md

example-status:
	@echo "==> $(EXAMPLE)"
	@sed -n '1,22p' $(EXAMPLE)/STATUS.md
