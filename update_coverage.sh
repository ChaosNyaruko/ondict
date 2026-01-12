#!/bin/bash

# Run tests and generate coverage profile
echo "Running tests..."
go test ./... -coverprofile=cover.out > /dev/null

# Extract total coverage
# Output example: "total: (statements) 25.0%"
COVERAGE_LINE=$(go tool cover -func cover.out | grep total)
COVERAGE=$(echo $COVERAGE_LINE | awk '{print $3}')
# Remove % symbol for calculation
VAL=${COVERAGE%\%}

# Determine color
COLOR=$(awk -v val="$VAL" 'BEGIN {
    if (val >= 80) print "green";
    else if (val >= 70) print "yellow";
    else if (val >= 50) print "orange";
    else print "red";
}')

echo "Total coverage: $COVERAGE, Color: $COLOR"

# Construct Badge URL
# URL encode % as %25 for the badge service
BADGE_URL="https://img.shields.io/badge/coverage-${VAL}%25-${COLOR}"

# Update README.md
# Works on both GNU sed and BSD sed (macOS)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s|!\[Coverage\](https://img.shields.io/badge/coverage-.*)|![Coverage](${BADGE_URL})|" README.md
else
    sed -i "s|!\[Coverage\](https://img.shields.io/badge/coverage-.*)|![Coverage](${BADGE_URL})|" README.md
fi

echo "README.md updated with new coverage badge."
