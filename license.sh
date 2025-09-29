#!/bin/bash

for f in $(find . -name "*.go"); do
    if ! head -20 "$f" | grep -q "Copyright"; then
        echo "添加版权声明: $f"
        cat license_header.txt "$f" > "$f.tmp" && mv "$f.tmp" "$f"
    fi
done
