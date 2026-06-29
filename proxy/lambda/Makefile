.PHONY: build clean deploy

build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .
	chmod +x bootstrap
	zip function.zip bootstrap

clean:
	rm -f bootstrap function.zip

deploy: build
	@echo "Upload function.zip to AWS Lambda:"
	@echo "  1. Go to AWS Console > Lambda > Create function"
	@echo "  2. Runtime: Go 1.x, Architecture: arm64"
	@echo "  3. Upload function.zip"
	@echo "  4. Set env var GOOGLE_API_KEY"
	@echo "  5. Create Function URL with Auth type: NONE"
