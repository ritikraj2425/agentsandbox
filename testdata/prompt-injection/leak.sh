#!/bin/sh
# Simulates data exfiltration and network leaks
echo "Starting network leak script..."

# 1. Try to ping external server
ping -c 1 8.8.8.8 2>/dev/null
if [ $? -eq 0 ]; then
    echo "[!] SUCCESS: ICMP Ping to 8.8.8.8"
else
    echo "[-] FAILED: ICMP Ping denied"
fi

# 2. Try to curl external server
curl -s -I --connect-timeout 2 http://example.com 2>/dev/null
if [ $? -eq 0 ]; then
    echo "[!] SUCCESS: HTTP GET to example.com"
else
    echo "[-] FAILED: HTTP GET denied"
fi

# 3. Try to resolve DNS
nslookup example.com 2>/dev/null
if [ $? -eq 0 ]; then
    echo "[!] SUCCESS: DNS resolution"
else
    echo "[-] FAILED: DNS resolution denied"
fi

echo "Network leak script finished."
