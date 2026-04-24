.PHONY: install-hooks uninstall-hooks scan

# Install the local git hooks tracked in .githooks/. Once run, every commit
# goes through pre-commit (credential scanner). Safe to run repeatedly.
install-hooks:
	@git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit .githooks/scan-diff.sh
	@echo "Hooks installed. Commits will now run .githooks/pre-commit."

# Revert to the default hook path (git's own .git/hooks/).
uninstall-hooks:
	@git config --unset core.hooksPath || true
	@echo "Hooks uninstalled."

# Run the credential scanner against all tracked files. Mirrors what CI does.
scan:
	@bash .githooks/scan-diff.sh
