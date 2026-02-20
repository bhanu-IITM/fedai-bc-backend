Systemd env file is at:

sudo nano /etc/golang-fabric-service/env


Safe Deploy Sequence:

sudo systemctl stop golang-fabric-service
sudo cp golang-fabric-service /opt/golang-fabric-service/golang-fabric-service
sudo systemctl start golang-fabric-service

Then:

sudo systemctl status golang-fabric-service --no-pager

3) Quick sanity test (health + one eval)

Since it’s bound to localhost, test from the VM:

Health
curl -s http://127.0.0.1:8080/healthz && echo