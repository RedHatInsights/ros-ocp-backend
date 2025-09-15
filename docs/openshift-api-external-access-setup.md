# OpenShift API and Console External Access Setup Guide

This document provides step-by-step instructions for configuring external access to OpenShift API servers (port 6443) and console (port 443) on RHEL 9.x bare metal hosts using kcli/libvirt deployment.

## Overview

When deploying OpenShift on RHEL 9.x with kcli/libvirt, both the API servers and application router run within VMs on an internal network (typically `192.168.122.0/24`). To provide external access to both the API and console, we need to:

1. Install and configure HAProxy as a load balancer for both API and console traffic
2. Configure iptables/firewalld to allow external traffic
3. Add specific libvirt firewall rules to permit new connections
4. Remove conflicting NAT rules that may redirect traffic incorrectly
5. Set up DNS resolution for both API and console hostnames

## Prerequisites

- OpenShift cluster deployed on RHEL 9.x using kcli with libvirt/KVM
- Root access to the bare metal host
- Knowledge of control plane VM IP addresses (for API access)
- Knowledge of worker node VM IP addresses (for console access)
- External hostnames configured:
  - API: `api.ocp-cluster.domain.com`
  - Console: `console-openshift-console.apps.ocp-cluster.domain.com`

## Step 1: Gather Cluster Information

First, collect the necessary information about your OpenShift cluster:

```bash
# List all VMs
virsh list --all

# Get IP addresses of control plane nodes (for API access)
virsh domifaddr ocp-cluster-ctlplane-0
virsh domifaddr ocp-cluster-ctlplane-1
virsh domifaddr ocp-cluster-ctlplane-2

# Get IP addresses of worker nodes (for console access)
virsh domifaddr ocp-cluster-worker-0
virsh domifaddr ocp-cluster-worker-1
virsh domifaddr ocp-cluster-worker-2

# Test internal API connectivity
curl -k https://CONTROL_PLANE_IP:6443/healthz

# Test internal console connectivity (should return HTTP 200)
curl -k -H "Host: console-openshift-console.apps.ocp-cluster.domain.com" https://WORKER_IP:443/ -I
```

**Example output:**
```
Control Plane IPs:
ocp-cluster-ctlplane-0: 192.168.122.22
ocp-cluster-ctlplane-1: 192.168.122.179
ocp-cluster-ctlplane-2: 192.168.122.155

Worker Node IPs:
ocp-cluster-worker-0: 192.168.122.84
ocp-cluster-worker-1: 192.168.122.6
ocp-cluster-worker-2: 192.168.122.31
```

## Step 2: Install and Configure HAProxy

### Install HAProxy
```bash
dnf install -y haproxy
```

### Configure HAProxy
Create the HAProxy configuration file for both API and console access:

```bash
cat > /etc/haproxy/haproxy.cfg << 'EOF'
global
    daemon
    chroot /var/lib/haproxy
    user haproxy
    group haproxy
    pidfile /var/run/haproxy.pid

defaults
    mode tcp
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

# API Server (port 6443)
frontend api_frontend
    bind *:6443
    default_backend api_backend

backend api_backend
    balance roundrobin
    server cp1 CONTROL_PLANE_1_IP:6443 check
    server cp2 CONTROL_PLANE_2_IP:6443 check
    server cp3 CONTROL_PLANE_3_IP:6443 check

# HTTPS Router (port 443) for Console and Applications
frontend https_frontend
    bind *:443
    default_backend https_backend

backend https_backend
    balance roundrobin
    server worker1 WORKER_1_IP:443 check
    server worker2 WORKER_2_IP:443 check

# HTTP Router (port 80) for HTTP redirects
frontend http_frontend
    bind *:80
    default_backend http_backend

backend http_backend
    balance roundrobin
    server worker1 WORKER_1_IP:80 check
    server worker2 WORKER_2_IP:80 check
EOF
```

**Replace the placeholders with your actual IPs:**
```bash
# Control plane IPs for API access
sed -i 's/CONTROL_PLANE_1_IP/192.168.122.22/g' /etc/haproxy/haproxy.cfg
sed -i 's/CONTROL_PLANE_2_IP/192.168.122.179/g' /etc/haproxy/haproxy.cfg
sed -i 's/CONTROL_PLANE_3_IP/192.168.122.155/g' /etc/haproxy/haproxy.cfg

# Worker node IPs for console access
sed -i 's/WORKER_1_IP/192.168.122.84/g' /etc/haproxy/haproxy.cfg
sed -i 's/WORKER_2_IP/192.168.122.6/g' /etc/haproxy/haproxy.cfg
```

### Configure SELinux for HAProxy
```bash
# Allow HAProxy to connect to any port
setsebool -P haproxy_connect_any 1

# Add port 6443 to allowed HTTP ports
semanage port -a -t http_port_t -p tcp 6443
```

### Start HAProxy
```bash
systemctl enable --now haproxy
systemctl status haproxy
```

## Step 3: Configure Firewalld

### Enable required ports
```bash
# Start firewalld if not running
systemctl start firewalld

# Add required ports
firewall-cmd --add-port=6443/tcp --permanent
firewall-cmd --add-port=443/tcp --permanent
firewall-cmd --add-port=80/tcp --permanent

# Enable masquerading
firewall-cmd --add-masquerade --permanent

# Apply changes
firewall-cmd --reload
```

### Remove any conflicting forward rules
```bash
# List existing forward rules
firewall-cmd --list-forward-ports

# Remove any old forward rules pointing to wrong IPs
# Example:
# firewall-cmd --remove-forward-port=port=6443:proto=tcp:toaddr=OLD_IP:toport=6443
```

## Step 4: Configure Libvirt Iptables Rules

This is the **critical step** that's often missed. Libvirt creates iptables rules that block new connections to VMs from external sources.

### Check current libvirt rules
```bash
iptables -L LIBVIRT_FWI -v -n
```

You'll see something like:
```
Chain LIBVIRT_FWI (1 references)
 pkts bytes target     prot opt in     out     source               destination
   49  4121 ACCEPT     all  --  *      virbr0  0.0.0.0/0            192.168.122.0/24     ctstate RELATED,ESTABLISHED
    8   512 REJECT     all  --  *      virbr0  0.0.0.0/0            0.0.0.0/0            reject-with icmp-port-unreachable
```

### Add rules to allow new connections
```bash
# Allow new connections to control plane nodes on port 6443 (API)
iptables -I LIBVIRT_FWI 2 -p tcp --dport 6443 -d CONTROL_PLANE_1_IP -j ACCEPT
iptables -I LIBVIRT_FWI 3 -p tcp --dport 6443 -d CONTROL_PLANE_2_IP -j ACCEPT
iptables -I LIBVIRT_FWI 4 -p tcp --dport 6443 -d CONTROL_PLANE_3_IP -j ACCEPT

# Allow new connections to worker nodes on port 443 (Console)
iptables -I LIBVIRT_FWI 5 -p tcp --dport 443 -d WORKER_1_IP -j ACCEPT
iptables -I LIBVIRT_FWI 6 -p tcp --dport 443 -d WORKER_2_IP -j ACCEPT

# Allow new connections to worker nodes on port 80 (HTTP redirects)
iptables -I LIBVIRT_FWI 7 -p tcp --dport 80 -d WORKER_1_IP -j ACCEPT
iptables -I LIBVIRT_FWI 8 -p tcp --dport 80 -d WORKER_2_IP -j ACCEPT
```

**Example with actual IPs:**
```bash
# Control plane rules for API
iptables -I LIBVIRT_FWI 2 -p tcp --dport 6443 -d 192.168.122.22 -j ACCEPT
iptables -I LIBVIRT_FWI 3 -p tcp --dport 6443 -d 192.168.122.179 -j ACCEPT
iptables -I LIBVIRT_FWI 4 -p tcp --dport 6443 -d 192.168.122.155 -j ACCEPT

# Worker node rules for console
iptables -I LIBVIRT_FWI 5 -p tcp --dport 443 -d 192.168.122.84 -j ACCEPT
iptables -I LIBVIRT_FWI 6 -p tcp --dport 443 -d 192.168.122.6 -j ACCEPT
iptables -I LIBVIRT_FWI 7 -p tcp --dport 80 -d 192.168.122.84 -j ACCEPT
iptables -I LIBVIRT_FWI 8 -p tcp --dport 80 -d 192.168.122.6 -j ACCEPT
```

### Make iptables rules persistent
```bash
# Save current iptables rules
iptables-save > /etc/sysconfig/iptables

# Enable iptables service to restore rules on boot
systemctl enable iptables
```

## Step 5: Remove Conflicting NAT Rules

**CRITICAL**: Check for and remove any existing NAT rules that might conflict with HAProxy. This is often the cause of both port 443 and port 6443 connectivity issues.

```bash
# Check for existing NAT rules that might redirect traffic
iptables -t nat -L PREROUTING -v -n

# Look for rules like:
# DNAT tcp dpt:443 to:SOME_IP:443
# DNAT tcp dpt:6443 to:SOME_IP:6443

# Remove conflicting rules if found (replace X with the rule number)
# iptables -t nat -D PREROUTING X
```

**IMPORTANT**: These DNAT rules bypass HAProxy completely and can cause:
- API server connectivity issues (port 6443)
- Console connectivity issues (port 443)
- Load balancing failures
- Single point of failure when targeting one specific VM

**Example of removing conflicting rules:**
```bash
# List rules with line numbers
iptables -t nat -L PREROUTING --line-numbers -n

# Remove by line number (safer than matching exact rule)
# If you see rule 2: "DNAT tcp dpt:6443 to:192.168.122.179:6443"
iptables -t nat -D PREROUTING 2

# If you see rule 3: "DNAT tcp dpt:443 to:192.168.122.253:443"
iptables -t nat -D PREROUTING 3
```

### Fix Connection Tracking Issues (Console Only)

**WARNING**: Only apply NOTRACK rules if console access fails after removing DNAT rules. Use specific rules to avoid breaking registry connectivity.

```bash
# Get the external IP of your host
EXTERNAL_IP=$(ip route get 8.8.8.8 | awk '{print $7; exit}')

# Add SPECIFIC NOTRACK rules for console access only
iptables -t raw -A PREROUTING -d ${EXTERNAL_IP} -p tcp --dport 443 -j NOTRACK
iptables -t raw -A OUTPUT -s ${EXTERNAL_IP} -p tcp --sport 443 -j NOTRACK
```

**CRITICAL WARNING**: Never use broad NOTRACK rules like:
```bash
# DON'T DO THIS - breaks VM registry connectivity
iptables -t raw -A PREROUTING -p tcp --dport 443 -j NOTRACK
iptables -t raw -A OUTPUT -p tcp --sport 443 -j NOTRACK
```

These broad rules disable connection tracking for ALL port 443 traffic, preventing OpenShift VMs from accessing external container registries (registry.access.redhat.com, quay.io, etc.).

## Step 6: Configure DNS Resolution

### Option A: Add to /etc/hosts (temporary)
```bash
# Get external IP of the host
EXTERNAL_IP=$(ip route get 8.8.8.8 | awk '{print $7; exit}')

# Add both API and console hostnames
echo "${EXTERNAL_IP} api.your-cluster.domain.com" >> /etc/hosts
echo "${EXTERNAL_IP} console-openshift-console.apps.your-cluster.domain.com" >> /etc/hosts
```

### Option B: Configure proper DNS
Add A records in your DNS server pointing both hostnames to the bare metal host's external IP:
- `api.your-cluster.domain.com`
- `console-openshift-console.apps.your-cluster.domain.com`

## Step 7: Test Configuration

### Test local connectivity
```bash
# Test HAProxy locally
curl -k https://127.0.0.1:6443/healthz

# Test external IP locally
curl -k https://EXTERNAL_IP:6443/healthz
```

### Test external connectivity
```bash
# Test API access from external client
curl -k https://api.your-cluster.domain.com:6443/healthz
# Should return: ok

# Test console access from external client
curl -k https://console-openshift-console.apps.your-cluster.domain.com/ -I
# Should return: HTTP/1.1 200 OK

# Test HTTP redirect
curl http://console-openshift-console.apps.your-cluster.domain.com/ -I
# Should return: HTTP/1.1 302 Found with location header pointing to HTTPS
```

### Test API endpoints
```bash
# Test actual API access (will show auth error, which is expected)
curl -k https://api.your-cluster.domain.com:6443/api/v1
```

## Troubleshooting

### 1. Check if packets are reaching the host
```bash
# Monitor for API traffic
tcpdump -i EXTERNAL_INTERFACE port 6443

# Monitor for console traffic
tcpdump -i EXTERNAL_INTERFACE port 443
```

### 2. Verify HAProxy is listening on all required ports
```bash
ss -tlnp | grep -E ':(80|443|6443)'
```

### 3. Check HAProxy logs
```bash
journalctl -u haproxy -f
```

### 4. Check for conflicting NAT rules
```bash
# This is the most common cause of port 443 issues
iptables -t nat -L PREROUTING -v -n
```

Look for DNAT rules that might redirect port 443 or 6443 traffic to wrong destinations.

### 5. Verify libvirt iptables rules
```bash
iptables -L LIBVIRT_FWI -v -n
```

Look for REJECT rules that might be blocking traffic.

### 6. Check firewalld configuration
```bash
firewall-cmd --list-all
```

### 7. Test backend connectivity
```bash
# Test each control plane node individually (API)
curl -k https://192.168.122.22:6443/healthz
curl -k https://192.168.122.179:6443/healthz
curl -k https://192.168.122.155:6443/healthz

# Test worker nodes individually (Console)
curl -k -H "Host: console-openshift-console.apps.cluster.domain.com" https://192.168.122.84:443/ -I
curl -k -H "Host: console-openshift-console.apps.cluster.domain.com" https://192.168.122.6:443/ -I
```

### 8. Debug specific port 443 issues
```bash
# Create a simple test web server on port 443 to isolate HAProxy issues
python3 -c "
import http.server
import ssl
import tempfile
import os

# Create temp directory and simple page
os.chdir(tempfile.mkdtemp())
with open('index.html', 'w') as f:
    f.write('<h1>Test Server</h1>')

# Generate self-signed cert
os.system('openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 1 -nodes -subj "/CN=test"')

# Start server
httpd = http.server.HTTPServer(('0.0.0.0', 8443), http.server.SimpleHTTPRequestHandler)
context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
context.load_cert_chain('cert.pem', 'key.pem')
httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
print('Test server running on port 8443...')
httpd.serve_forever()
"

# Test if alternate port works:
curl -k https://HOST_IP:8443/
```

### 9. Debug OpenShift Registry Connectivity Issues
```bash
# Test registry connectivity from VMs
ssh core@VM_IP "curl -I https://registry.access.redhat.com"

# Check for broad NOTRACK rules that break outbound HTTPS
iptables -t raw -L -n | grep "tcp dpt:443"

# Test VM outbound connectivity
ssh core@VM_IP "ping -c 2 8.8.8.8"  # Should work
ssh core@VM_IP "curl -I https://google.com"  # Should work if no NOTRACK issues

# Check OpenShift pod image pull status
oc get pods --all-namespaces | grep -E "(ImagePull|ErrImage)"
oc describe pod POD_NAME -n NAMESPACE  # Check events for pull errors

# If VMs can't reach external registries, check iptables raw table:
iptables -t raw -L PREROUTING -n
# Look for broad NOTRACK rules without destination IP that affect all port 443
# Remove them: iptables -t raw -D PREROUTING X

# Test connectivity after fixes
ssh core@VM_IP "timeout 10 nc -zv registry.access.redhat.com 443"
```

## Template Script for New Deployments

Here's a template script that can be customized for new cluster deployments:

```bash
#!/bin/bash

# Configuration variables - CUSTOMIZE THESE
CLUSTER_NAME="ocp-cluster"
API_HOSTNAME="api.${CLUSTER_NAME}.domain.com"
CONSOLE_HOSTNAME="console-openshift-console.apps.${CLUSTER_NAME}.domain.com"
CONTROL_PLANE_IPS=("192.168.122.22" "192.168.122.179" "192.168.122.155")
WORKER_IPS=("192.168.122.84" "192.168.122.6")

# Get external IP of host
EXTERNAL_IP=$(ip route get 8.8.8.8 | awk '{print $7; exit}')

echo "Setting up external access for ${CLUSTER_NAME}"
echo "External IP: ${EXTERNAL_IP}"
echo "API Hostname: ${API_HOSTNAME}"
echo "Console Hostname: ${CONSOLE_HOSTNAME}"
echo "Control Plane IPs: ${CONTROL_PLANE_IPS[@]}"
echo "Worker IPs: ${WORKER_IPS[@]}"

# Install HAProxy
dnf install -y haproxy

# Generate HAProxy config
cat > /etc/haproxy/haproxy.cfg << EOF
global
    daemon
    chroot /var/lib/haproxy
    user haproxy
    group haproxy
    pidfile /var/run/haproxy.pid

defaults
    mode tcp
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

# API Server (port 6443)
frontend api_frontend
    bind *:6443
    default_backend api_backend

backend api_backend
    balance roundrobin
$(for i in "${!CONTROL_PLANE_IPS[@]}"; do
    echo "    server cp$((i+1)) ${CONTROL_PLANE_IPS[$i]}:6443 check"
done)

# HTTPS Router (port 443) for Console
frontend https_frontend
    bind *:443
    default_backend https_backend

backend https_backend
    balance roundrobin
$(for i in "${!WORKER_IPS[@]}"; do
    echo "    server worker$((i+1)) ${WORKER_IPS[$i]}:443 check"
done)

# HTTP Router (port 80)
frontend http_frontend
    bind *:80
    default_backend http_backend

backend http_backend
    balance roundrobin
$(for i in "${!WORKER_IPS[@]}"; do
    echo "    server worker$((i+1)) ${WORKER_IPS[$i]}:80 check"
done)
EOF

# Configure SELinux
setsebool -P haproxy_connect_any 1
semanage port -a -t http_port_t -p tcp 6443 2>/dev/null || true

# Configure firewall
systemctl start firewalld
firewall-cmd --add-port=6443/tcp --permanent
firewall-cmd --add-port=443/tcp --permanent
firewall-cmd --add-port=80/tcp --permanent
firewall-cmd --add-masquerade --permanent
firewall-cmd --reload

# Check for and remove any conflicting NAT rules
echo "Checking for conflicting NAT rules..."
iptables -t nat -L PREROUTING -v -n | grep -E "(443|6443)" && echo "WARNING: Found existing NAT rules - manual review needed"

# Add libvirt iptables rules for API
for i in "${!CONTROL_PLANE_IPS[@]}"; do
    iptables -I LIBVIRT_FWI $((i+2)) -p tcp --dport 6443 -d "${CONTROL_PLANE_IPS[$i]}" -j ACCEPT
done

# Add libvirt iptables rules for console
for i in "${!WORKER_IPS[@]}"; do
    iptables -I LIBVIRT_FWI $((i+5)) -p tcp --dport 443 -d "${WORKER_IPS[$i]}" -j ACCEPT
    iptables -I LIBVIRT_FWI $((i+7)) -p tcp --dport 80 -d "${WORKER_IPS[$i]}" -j ACCEPT
done

# Save iptables
iptables-save > /etc/sysconfig/iptables

# Start HAProxy
systemctl enable --now haproxy

# Add DNS entries
echo "${EXTERNAL_IP} ${API_HOSTNAME}" >> /etc/hosts
echo "${EXTERNAL_IP} ${CONSOLE_HOSTNAME}" >> /etc/hosts

echo "Setup complete! Test with:"
echo "curl -k https://${API_HOSTNAME}:6443/healthz"
echo "curl -k https://${CONSOLE_HOSTNAME}/ -I"
```

## Security Considerations

1. **Firewall Rules**: Only open ports 6443 and 443 as needed
2. **Network Segmentation**: Consider using VLANs or separate networks for management traffic
3. **Certificate Management**: Use proper TLS certificates instead of self-signed ones in production
4. **Access Control**: Implement proper RBAC and authentication on the OpenShift cluster
5. **Monitoring**: Set up monitoring for HAProxy and the API endpoints

## Common Issues and Solutions

| Issue | Symptoms | Solution |
|-------|----------|----------|
| Port 443 not accessible | `curl: (7) Failed to connect...` on 443 | Check for conflicting NAT rules in `iptables -t nat -L PREROUTING` |
| Port 6443 not accessible | `curl: (7) Failed to connect...` on 6443 | Remove DNAT rules that bypass HAProxy: `iptables -t nat -D PREROUTING X` |
| Connection refused | `curl: (7) Failed to connect...` | Check if HAProxy is running and listening on required ports |
| Connection timeout | `curl: (28) Operation timed out` | Check firewall rules and iptables LIBVIRT_FWI chain |
| Console shows 503 error | HTTP 503 Service Unavailable | Verify worker node IPs and that router pods are running |
| Permission denied | HAProxy fails to start | Configure SELinux with `setsebool -P haproxy_connect_any 1` |
| DNS resolution fails | `server can't find hostname` | Add entries to /etc/hosts or configure proper DNS |
| Backend connection fails | HAProxy logs show backend errors | Verify VM IPs and status |
| API works but console doesn't | API accessible, console 503/timeout | Check worker node IPs and router configuration |
| Registry image pull failures | `dial tcp: i/o timeout` on registry.access.redhat.com | Remove broad NOTRACK rules affecting port 443 |
| VM outbound HTTPS broken | External HTTPS sites unreachable from VMs | Check for overly broad NOTRACK rules in raw table |

## Important Notes for RHEL 9.x + kcli Deployments

- **NAT Rule Conflicts**: The most common issue is existing iptables NAT rules that redirect port 443 or 6443 traffic directly to VMs, bypassing HAProxy
- **DNAT Rules Bypass HAProxy**: Any DNAT rules in the PREROUTING chain will redirect traffic before it reaches HAProxy, breaking load balancing
- **Connection Tracking**: Port 443 may require connection tracking to be disabled, but ONLY use specific NOTRACK rules targeting the external host IP
- **Broad NOTRACK Rules Break Registry Access**: Never use `iptables -t raw -A PREROUTING -p tcp --dport 443 -j NOTRACK` without destination IP - it prevents VM outbound HTTPS
- **Worker vs Control Plane**: Console traffic goes to worker nodes (443/80), API traffic goes to control planes (6443)
- **Router Dependencies**: Console access depends on OpenShift router pods running on worker nodes
- **Registry Connectivity**: OpenShift VMs need outbound HTTPS access to external registries; overly broad iptables rules can break this

This documentation provides complete setup instructions for exposing both OpenShift API (port 6443) and console (port 443) from RHEL 9.x bare metal hosts using kcli deployments.