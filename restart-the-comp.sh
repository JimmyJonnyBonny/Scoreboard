#!/bin/bash

echo "[*] Stopping DWAYNE-INATOR-5000..."
pkill -f DWAYNE-INATOR-5000

echo "[*] Removing old database..."
rm -f dwayne.db

echo "[*] Removing old injects config..."
rm -rf injects.conf

echo "[*] Starting DWAYNE-INATOR-5000..."
./DWAYNE-INATOR-5000
