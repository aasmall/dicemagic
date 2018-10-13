#!/bin/bash

secs=300                         # Set interval (duration) in seconds.
endTime=$(( $(date +%s) + secs )) # Calculate end time.

while [ $(date +%s) -lt $endTime ]; do  # Loop until interval has elapsed.
    curl "https://api.k8s.dicemagic.io/roll?cmd=roll%201d20%20red%20and%208d8%20blue"
done