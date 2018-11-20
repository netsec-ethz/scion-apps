#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# define threshold
min_sec=15

echo "Checking time using google.com now:"
tl=$(date '+%s')
ts=$(date '+%s' --date="$(curl -sI google.com | sed -n  '/Date:\s*/s///p')")
diff=$((ts-tl))
diff=${diff#-} # abs(diff)
echo Time diff: "$diff"s
if [ $diff -gt $min_sec ]; then
    echo "Offset must be within $min_sec seconds."
    exit 1
fi
