#!/bin/bash
#
# Script to add license headers to Go source files that don't have them.
# Run this script from the repository root directory.
#
# Usage: ./scripts/add-license-headers.sh [--check]
#   --check: Only check for missing headers, don't modify files (exit 1 if any missing)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
YEAR=$(date +%Y)

# License header template (without the leading newline)
LICENSE_HEADER="/*
 * Copyright $YEAR Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the \"License\"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an \"AS IS\" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */
"

CHECK_ONLY=false
if [[ "$1" == "--check" ]]; then
    CHECK_ONLY=true
fi

MISSING_HEADERS=()

# Find all Go files, excluding vendor directory
while IFS= read -r -d '' file; do
    # Check if file already has a license header (starts with /*)
    if head -1 "$file" | grep -q "^/\*"; then
        continue
    fi
    
    MISSING_HEADERS+=("$file")
    
    if [[ "$CHECK_ONLY" == false ]]; then
        echo "Adding license header to: $file"
        
        # Create temp file with license header + original content
        TEMP_FILE=$(mktemp)
        echo "$LICENSE_HEADER" > "$TEMP_FILE"
        echo "" >> "$TEMP_FILE"  # Add blank line after header
        cat "$file" >> "$TEMP_FILE"
        
        # Replace original file
        mv "$TEMP_FILE" "$file"
    fi
done < <(find "$ROOT_DIR" -name "*.go" -type f ! -path "*/vendor/*" -print0)

if [[ ${#MISSING_HEADERS[@]} -gt 0 ]]; then
    if [[ "$CHECK_ONLY" == true ]]; then
        echo "Files missing license headers:"
        for file in "${MISSING_HEADERS[@]}"; do
            echo "  - $file"
        done
        echo ""
        echo "Run './scripts/add-license-headers.sh' to add headers."
        exit 1
    else
        echo ""
        echo "Added license headers to ${#MISSING_HEADERS[@]} file(s)."
    fi
else
    echo "All Go files have license headers."
fi
