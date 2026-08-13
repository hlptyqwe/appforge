.PHONY: fmt build
fmt:
	npx prettier --write "src/**/*.{ts,tsx,js,jsx,json,css,scss,md}"
	npx prettier --write "src/components/**/*.vue"
	npx prettier --write "src/views/**/*.vue"
	npx eslint "src/components/**/*.vue" --fix
	npx eslint "src/views/**/*.vue" --fix

build:
	yarn run build