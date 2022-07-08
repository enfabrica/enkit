#!/usr/bin/python3

import os
import subprocess
import sys

filename=sys.argv[1]
want=sys.argv[2]

if not os.path.exists(filename):
    print(f"{filename} is missing")
    sys.exit(1)

cmd = ["/usr/bin/sum", filename]
output = subprocess.check_output(cmd).decode('utf-8')
print(cmd)
print(output)
print(want)

got = output.split()[0]
if got != want:
    print("Checksums did not match!")
    print(f"  Want: {want}")
    print(f"   Got: {got}")
    sys.exit(1)

print("Checksum matched.")
sys.exit(0)

