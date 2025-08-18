#!/bin/bash

# Set host and port for the API server
HOST="localhost"
PORT="8080"

# Test script for verifying the wilayah-indonesia API endpoints
# Make sure the API server is running on HOST:PORT before running this script

echo "Testing wilayah-indonesia API endpoints"
echo "======================================"

# Test 1: General search endpoint with successful result
echo -e "\n1. Testing general search endpoint with successful result:"
curl -s "http://${HOST}:${PORT}/v1/search?q=jakarta" | jq '.'

# Test 2: General search endpoint with no results
echo -e "\n2. Testing general search endpoint with no results:"
curl -s "http://${HOST}:${PORT}/v1/search?q=nonexistentplace123" | jq '.'

# Test 3: District search endpoint with successful result
echo -e "\n3. Testing district search endpoint with successful result:"
curl -s "http://${HOST}:${PORT}/v1/search/district?q=bandung" | jq '.'

# Test 4: District search endpoint with no results
echo -e "\n4. Testing district search endpoint with no results:"
curl -s "http://${HOST}:${PORT}/v1/search/district?q=nonexistentdistrict123" | jq '.'

# Test 5: Subdistrict search endpoint with successful result
echo -e "\n5. Testing subdistrict search endpoint with successful result:"
curl -s "http://${HOST}:${PORT}/v1/search/subdistrict?q=menteng" | jq '.'

# Test 6: Subdistrict search endpoint with no results
echo -e "\n6. Testing subdistrict search endpoint with no results:"
curl -s "http://${HOST}:${PORT}/v1/search/subdistrict?q=nonexistentsubdistrict123" | jq '.'

# Test 7: City search endpoint with successful result
echo -e "\n7. Testing city search endpoint with successful result:"
curl -s "http://${HOST}:${PORT}/v1/search/city?q=surabaya" | jq '.'

# Test 8: City search endpoint with no results
echo -e "\n8. Testing city search endpoint with no results:"
curl -s "http://${HOST}:${PORT}/v1/search/city?q=nonexistentcity123" | jq '.'

# Test 9: Province search endpoint with successful result
echo -e "\n9. Testing province search endpoint with successful result:"
curl -s "http://${HOST}:${PORT}/v1/search/province?q=jawa%20tengah" | jq '.'

# Test 10: Province search endpoint with no results
echo -e "\n10. Testing province search endpoint with no results:"
curl -s "http://${HOST}:${PORT}/v1/search/province?q=nonexistentprovince123" | jq '.'

# Test 11: Testing endpoint without required query parameter
echo -e "\n11. Testing endpoint without required query parameter:"
curl -s "http://${HOST}:${PORT}/v1/search" | jq '.'
# Test 12: Postal code search endpoint with successful result
echo -e "\n12. Testing postal code search endpoint with successful result:"
curl -s "http://${HOST}:${PORT}/v1/search/postal/10110" | jq '.'

# Test 13: Postal code search endpoint with invalid postal code format
echo -e "\n13. Testing postal code search endpoint with invalid postal code format:"
curl -s "http://${HOST}:${PORT}/v1/search/postal/123" | jq '.'

# Test 14: Postal code search endpoint with non-existent postal code
echo -e "\n14. Testing postal code search endpoint with non-existent postal code:"
curl -s "http://${HOST}:${PORT}/v1/search/postal/99999" | jq '.'

echo -e "\n\nTest completed!"