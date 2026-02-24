mkdir -p ~/services
cd ~/services
git clone https://github.com/bhanu-IITM/fedai-bc-backend.git golang-fabric-service
cd golang-fabric-service

cd ~/services/golang-fabric-service
go mod tidy
go build -o golang-fabric-service ./cmd/server   # adjust path if your main is elsewhere


# (Optional) put binary in a stable location:
sudo mkdir -p /opt/golang-fabric-service
sudo cp ./golang-fabric-service /opt/golang-fabric-service/


sudo cp -r ./configs /opt/golang-fabric-service/ 2>/dev/null || true

#2) Create an env file (paths + peer endpoint)
sudo mkdir -p /etc/golang-fabric-service
sudo nano /etc/golang-fabric-service/env

Put (example — edit to your real paths):

'''
PORT=8080
PEER_ENDPOINT=localhost:7051
GATEWAY_PEER=peer0.org1.example.com
CHANNEL_NAME=mychannel
CHAINCODE_NAME=basicccaas
MSP_ID=Org1MSP

# identity used by gateway service (admin or app user)
CERT_PATH=/home/bhanu/fabric/two-vm/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/cert.pem
KEY_PATH=/home/bhanu/fabric/two-vm/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/<KEY_FILE>.pem

# peer TLS CA (if TLS enabled)
TLS_CERT_PATH=/home/bhanu/fabric/two-vm/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
TLS_ENABLED=true
'''

# Find the key filename:
ls -1 ~/fabric/two-vm/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore
Copy that name into KEY_PATH.

3) Create the systemd unit
sudo nano /etc/systemd/system/fabric-gateway-service.service

4) Logs:
journalctl -u golang-fabric-service -f



Systemd env file is at:
sudo nano /etc/golang-fabric-service/env


Safe Deploy Sequence:

sudo systemctl stop golang-fabric-service
sudo cp golang-fabric-service /opt/golang-fabric-service/golang-fabric-service
sudo systemctl start golang-fabric-service

# to restart after changes:
sudo systemctl restart golang-fabric-service

Then:

sudo systemctl status golang-fabric-service --no-pager

3) Quick sanity test (health + one eval)

Since it’s bound to localhost, test from the VM:

Health
curl -s http://127.0.0.1:8080/healthz && echo



### Deploying and testing the CA service from postman:
{ "error": "mkdir admin keystore: mkdir /opt/golang-fabric-service/identities: permission denied", "status": "FAILED" }

give the service user ownership of the identities dir

On the VM, run:

# 1) Check which user the service runs as
systemctl cat golang-fabric-service | sed -n '1,120p' | egrep -i 'User=|Group=' || true

# If it shows something like User=bhanu or User=golang, use that below.

Now create + assign permissions:

# 2) Create directories
sudo mkdir -p /opt/golang-fabric-service/identities

# 3) Replace <USER> and <GROUP> with the service user/group (common: bhanu:bhanu)
sudo chown -R <USER>:<GROUP> /opt/golang-fabric-service/identities

# 4) Allow group write (optional but good)
sudo chmod -R 775 /opt/golang-fabric-service/identities

Restart:

sudo systemctl restart golang-fabric-service
sudo journalctl -u golang-fabric-service -n 80 --no-pager

Then retry /ca/enroll-admin from Postman.



Now then; we get this issue:
{ "error": "ca init: Failed to get client TLS config: Failed to read '/opt/golang-fabric-service/certs/ca-org1/tls-cert.pem': open /opt/golang-fabric-service/certs/ca-org1/tls-cert.pem: no such file or directory", "status": "FAILED" }

Your CA is running as ca-org1 on 7054, and your code expects the CA TLS cert at:

/opt/golang-fabric-service/certs/ca-org1/tls-cert.pem

…but that file doesn’t exist.

1) First, locate the CA TLS cert on the VM (source of truth)

On the peer VM, run:

# Find likely CA TLS certs in your two-vm workspace
ls -l ~/fabric/two-vm/organizations/fabric-ca/org1/tls-cert.pem 2>/dev/null || true
ls -l ~/fabric/two-vm/organizations/fabric-ca/org1/ca-cert.pem  2>/dev/null || true
ls -l ~/fabric/two-vm/organizations/fabric-ca/org1/tls/ 2>/dev/null || true

# Also check inside container (very reliable)
docker exec -it ca_org1 sh -lc 'ls -l /etc/hyperledger/fabric-ca-server/ && ls -l /etc/hyperledger/fabric-ca-server-config/'

What you want is the CA server TLS cert (or CA chain/root) file that the client should trust.

Common filenames you’ll see:

tls-cert.pem
ca-cert.pem
ca.org1.example.com-cert.pem
fabric-ca-server-tls.pem

If you see a file like /etc/hyperledger/fabric-ca-server-config/ca-cert.pem or tls-cert.pem inside the container, that’s the one to copy out.

2) Copy the CA TLS cert to the exact path your service expects
Option A (simple): copy from your repo’s generated crypto (preferred if present)

If this exists:
~/fabric/two-vm/organizations/fabric-ca/org1/tls-cert.pem

Then do:

sudo mkdir -p /opt/golang-fabric-service/certs/ca-org1
sudo cp ~/fabric/two-vm/organizations/fabric-ca/org1/tls-cert.pem /opt/golang-fabric-service/certs/ca-org1/tls-cert.pem
sudo chmod 644 /opt/golang-fabric-service/certs/ca-org1/tls-cert.pem

3) Ensure env points to the correct path

Open your env file:

sudo nano /etc/golang-fabric-service/env

Set (or fix):

FABRIC_CA_TLS_CERT=/opt/golang-fabric-service/certs/ca-org1/tls-cert.pem
FABRIC_CA_URL=https://127.0.0.1:7054

Then:

sudo systemctl restart golang-fabric-service
sudo journalctl -u golang-fabric-service -n 80 --no-pager

Sanity check:

sudo test -f /opt/golang-fabric-service/certs/ca-org1/tls-cert.pem && echo "CA TLS CERT OK"

4) Retry Postman

POST http://<PUBLIC_IP>:8080/ca/enroll-admin
It should now succeed.



