#!/usr/bin/env bash
set -euo pipefail

# Prerequisites:
#   1. Azure CLI installed (https://aka.ms/azure-cli)
#   2. Run: az login
#   3. Set your unique app name below
#
# Usage: bash deploy.sh

APP_NAME="semantic-cache-demo"
RESOURCE_GROUP="semantic-cache-rg"
LOCATION="eastus"

echo "=== Building Go binary ==="
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o handler .
echo "done."

echo "=== Creating resource group ==="
az group create --name "$RESOURCE_GROUP" --location "$LOCATION" -o none

echo "=== Creating storage account (required by Functions) ==="
STORAGE_NAME="${APP_NAME//-/}storage"
az storage account create --name "$STORAGE_NAME" --location "$LOCATION" \
  --resource-group "$RESOURCE_GROUP" --sku Standard_LRS --kind StorageV2 -o none

echo "=== Creating Function App (Linux, Go custom handler) ===""
az functionapp create --name "$APP_NAME" --resource-group "$RESOURCE_GROUP" \
  --storage-account "$STORAGE_NAME" \
  --consumption-plan-location "$LOCATION" \
  --os-type Linux --runtime custom --functions-version 4 -o none

echo "=== Setting GOOGLE_API_KEY env var ==="
read -sp "Enter your Gemini API key: " API_KEY
echo
az functionapp config appsettings set --name "$APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --settings GOOGLE_API_KEY="$API_KEY" -o none

echo "=== Deploying function ==="
func azure functionapp publish "$APP_NAME" --no-build

echo ""
echo "=== DONE ==="
echo "Your function URL: https://$APP_NAME.azurewebsites.net/api/Chat"
echo ""
echo "Test with:"
echo "  curl -X POST https://$APP_NAME.azurewebsites.net/api/Chat \\"
echo '    -H "Content-Type: application/json" \'
echo '    -d "{\"prompt\":\"capital of France\"}"'
